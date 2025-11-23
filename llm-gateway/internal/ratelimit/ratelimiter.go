package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter is used to enforce per-key rate limits.
type Limiter interface {
	Allow(ctx context.Context, key string) bool
}

// NoopLimiter allows all requests (no rate limiting yet).
type NoopLimiter struct{}

func NewNoopLimiter() *NoopLimiter {
	return &NoopLimiter{}
}

func (l *NoopLimiter) Allow(ctx context.Context, key string) bool {
	return true
}

// RateLimiter implements distributed rate limiting using Redis
type RateLimiter struct {
	client *redis.Client
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

// Allow checks if a request should be allowed for the given key
// Uses sliding window algorithm with Redis sorted sets
func (rl *RateLimiter) Allow(ctx context.Context, apiKeyID string, limit int) (bool, error) {
	return rl.AllowN(ctx, apiKeyID, limit, 1)
}

// AllowN checks if N requests should be allowed for the given key
func (rl *RateLimiter) AllowN(ctx context.Context, apiKeyID string, limit int, count int) (bool, error) {
	if limit <= 0 {
		// No limit configured
		return true, nil
	}
	
	key := fmt.Sprintf("ratelimit:%s", apiKeyID)
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)
	
	pipe := rl.client.Pipeline()
	
	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli()))
	
	// Count current requests in window
	countCmd := pipe.ZCard(ctx, key)
	
	// Add current request(s) with timestamp as score
	for i := 0; i < count; i++ {
		timestamp := now.Add(time.Duration(i) * time.Microsecond).UnixMilli()
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(timestamp),
			Member: fmt.Sprintf("%d:%d", timestamp, i),
		})
	}
	
	// Set expiry on the key (cleanup old keys)
	pipe.Expire(ctx, key, 2*time.Minute)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("rate limit check failed: %w", err)
	}
	
	currentCount := countCmd.Val()
	
	// Check if adding these requests would exceed the limit
	return int(currentCount) <= limit, nil
}

// GetCurrentUsage returns the current request count in the window
func (rl *RateLimiter) GetCurrentUsage(ctx context.Context, apiKeyID string) (int64, error) {
	key := fmt.Sprintf("ratelimit:%s", apiKeyID)
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)
	
	// Remove old entries
	if err := rl.client.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixMilli())).Err(); err != nil {
		return 0, fmt.Errorf("failed to clean old entries: %w", err)
	}
	
	// Count current requests
	count, err := rl.client.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get current usage: %w", err)
	}
	
	return count, nil
}

// Reset resets the rate limit for a key
func (rl *RateLimiter) Reset(ctx context.Context, apiKeyID string) error {
	key := fmt.Sprintf("ratelimit:%s", apiKeyID)
	return rl.client.Del(ctx, key).Err()
}

// TokenBucketLimiter implements token bucket algorithm (alternative)
type TokenBucketLimiter struct {
	client *redis.Client
}

// NewTokenBucketLimiter creates a new token bucket rate limiter
func NewTokenBucketLimiter(client *redis.Client) *TokenBucketLimiter {
	return &TokenBucketLimiter{client: client}
}

// Allow checks if a request should be allowed using token bucket
func (tbl *TokenBucketLimiter) Allow(ctx context.Context, apiKeyID string, rate int, burst int) (bool, error) {
	key := fmt.Sprintf("tokenbucket:%s", apiKeyID)
	tokensKey := key + ":tokens"
	lastRefillKey := key + ":last"
	
	now := time.Now()
	
	// Lua script for atomic token bucket check and update
	script := redis.NewScript(`
		local tokens_key = KEYS[1]
		local last_refill_key = KEYS[2]
		local rate = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local cost = tonumber(ARGV[4])
		
		-- Get current tokens and last refill time
		local tokens = tonumber(redis.call('GET', tokens_key)) or burst
		local last_refill = tonumber(redis.call('GET', last_refill_key)) or now
		
		-- Calculate tokens to add based on time elapsed
		local elapsed = now - last_refill
		local tokens_to_add = (elapsed / 60000) * rate -- rate per minute
		tokens = math.min(burst, tokens + tokens_to_add)
		
		-- Check if we have enough tokens
		if tokens >= cost then
			tokens = tokens - cost
			redis.call('SET', tokens_key, tokens, 'EX', 120)
			redis.call('SET', last_refill_key, now, 'EX', 120)
			return 1
		else
			-- Update tokens but deny request
			redis.call('SET', tokens_key, tokens, 'EX', 120)
			redis.call('SET', last_refill_key, now, 'EX', 120)
			return 0
		end
	`)
	
	result, err := script.Run(
		ctx, 
		tbl.client,
		[]string{tokensKey, lastRefillKey},
		rate,
		burst,
		now.UnixMilli(),
		1, // cost per request
	).Int()
	
	if err != nil {
		return false, fmt.Errorf("token bucket check failed: %w", err)
	}
	
	return result == 1, nil
}

// GetRemainingTokens returns the number of tokens remaining
func (tbl *TokenBucketLimiter) GetRemainingTokens(ctx context.Context, apiKeyID string, rate int, burst int) (float64, error) {
	key := fmt.Sprintf("tokenbucket:%s", apiKeyID)
	tokensKey := key + ":tokens"
	lastRefillKey := key + ":last"
	
	now := time.Now()
	
	script := redis.NewScript(`
		local tokens_key = KEYS[1]
		local last_refill_key = KEYS[2]
		local rate = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		local tokens = tonumber(redis.call('GET', tokens_key)) or burst
		local last_refill = tonumber(redis.call('GET', last_refill_key)) or now
		
		local elapsed = now - last_refill
		local tokens_to_add = (elapsed / 60000) * rate
		tokens = math.min(burst, tokens + tokens_to_add)
		
		return tokens
	`)
	
	result, err := script.Run(
		ctx,
		tbl.client,
		[]string{tokensKey, lastRefillKey},
		rate,
		burst,
		now.UnixMilli(),
	).Float64()
	
	if err != nil {
		return 0, fmt.Errorf("failed to get remaining tokens: %w", err)
	}
	
	return result, nil
}

// Reset resets the token bucket for a key
func (tbl *TokenBucketLimiter) Reset(ctx context.Context, apiKeyID string) error {
	key := fmt.Sprintf("tokenbucket:%s", apiKeyID)
	pipe := tbl.client.Pipeline()
	pipe.Del(ctx, key+":tokens")
	pipe.Del(ctx, key+":last")
	_, err := pipe.Exec(ctx)
	return err
}
