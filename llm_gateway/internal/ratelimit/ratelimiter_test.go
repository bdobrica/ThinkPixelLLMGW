package ratelimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestRateLimiter_AllowWithDetails(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		client, mr := setupTestRedis(t)
		defer mr.Close()
		defer client.Close()

		limiter := NewRateLimiter(client)
		ctx := context.Background()

		apiKeyID := "test-key-1"
		limit := 5

		// Make 5 requests - should all be allowed
		for i := 0; i < 5; i++ {
			allowed, remaining, resetAt, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
			require.NoError(t, err)
			assert.True(t, allowed)
			assert.Equal(t, limit-i-1, remaining)
			assert.False(t, resetAt.IsZero())
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		client, mr := setupTestRedis(t)
		defer mr.Close()
		defer client.Close()

		limiter := NewRateLimiter(client)
		ctx := context.Background()

		apiKeyID := "test-key-2"
		limit := 3

		// Make 3 requests - should all be allowed
		for i := 0; i < 3; i++ {
			allowed, _, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
			require.NoError(t, err)
			assert.True(t, allowed)
		}

		// 4th request should be blocked
		allowed, remaining, resetAt, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
		assert.False(t, allowed)
		assert.Equal(t, 0, remaining)
		assert.False(t, resetAt.IsZero())
	})

	t.Run("unlimited when limit is 0", func(t *testing.T) {
		client, mr := setupTestRedis(t)
		defer mr.Close()
		defer client.Close()

		limiter := NewRateLimiter(client)
		ctx := context.Background()

		apiKeyID := "test-key-unlimited"
		limit := 0

		// Make many requests - should all be allowed
		for i := 0; i < 100; i++ {
			allowed, remaining, resetAt, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
			require.NoError(t, err)
			assert.True(t, allowed)
			assert.Equal(t, -1, remaining) // -1 indicates unlimited
			assert.True(t, resetAt.IsZero())
		}
	})

	t.Run("resets after window expires", func(t *testing.T) {
		client, mr := setupTestRedis(t)
		defer mr.Close()
		defer client.Close()

		limiter := NewRateLimiter(client)
		ctx := context.Background()

		apiKeyID := "test-key-window"
		limit := 2

		// Use up the limit
		allowed, _, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
		assert.True(t, allowed)

		allowed, _, _, err = limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
		assert.True(t, allowed)

		// Should be blocked
		allowed, _, _, err = limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
		assert.False(t, allowed)

		// Reset the limiter (simulates window expiry)
		err = limiter.Reset(ctx, apiKeyID)
		require.NoError(t, err)

		// Should be allowed again after reset
		allowed, remaining, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, limit-1, remaining)
	})
}

func TestRateLimiter_GetCurrentUsage(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter := NewRateLimiter(client)
	ctx := context.Background()

	apiKeyID := "test-usage"
	limit := 10

	// Initially should be 0
	usage, err := limiter.GetCurrentUsage(ctx, apiKeyID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage)

	// Make 3 requests
	for i := 0; i < 3; i++ {
		_, _, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
	}

	// Should show 3 requests
	usage, err = limiter.GetCurrentUsage(ctx, apiKeyID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), usage)
}

func TestRateLimiter_Reset(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter := NewRateLimiter(client)
	ctx := context.Background()

	apiKeyID := "test-reset"
	limit := 2

	// Use up the limit
	for i := 0; i < 2; i++ {
		allowed, _, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	// Should be blocked
	allowed, _, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
	require.NoError(t, err)
	assert.False(t, allowed)

	// Reset the limiter
	err = limiter.Reset(ctx, apiKeyID)
	require.NoError(t, err)

	// Should be allowed again
	allowed, remaining, _, err := limiter.AllowWithDetails(ctx, apiKeyID, limit)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, limit-1, remaining)
}

func TestNoopLimiter(t *testing.T) {
	limiter := NewNoopLimiter()
	ctx := context.Background()

	// Should always allow
	for i := 0; i < 100; i++ {
		allowed := limiter.Allow(ctx, "any-key")
		assert.True(t, allowed)
	}
}
