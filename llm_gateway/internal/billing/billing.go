package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"llm_gateway/internal/storage"
)

// Service tracks costs and enforces budgets.
type Service interface {
	WithinBudget(ctx context.Context, apiKeyID string) bool
	AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error
}

// NoopService does not enforce budgets and discards usage.
type NoopService struct{}

func NewNoopService() *NoopService {
	return &NoopService{}
}

func (s *NoopService) WithinBudget(ctx context.Context, apiKeyID string) bool {
	return true
}

func (s *NoopService) AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error {
	return nil
}

// RedisBillingService tracks costs in Redis and enforces budgets
type RedisBillingService struct {
	redis    *redis.Client
	db       *storage.DB
	syncFreq time.Duration // How often to sync Redis → DB
}

// NewRedisBillingService creates a new billing service
func NewRedisBillingService(redis *redis.Client, db *storage.DB, syncFrequency time.Duration) *RedisBillingService {
	service := &RedisBillingService{
		redis:    redis,
		db:       db,
		syncFreq: syncFrequency,
	}

	// Start background sync worker
	go service.syncWorker()

	return service
}

// WithinBudget checks if an API key is within its monthly budget
func (s *RedisBillingService) WithinBudget(ctx context.Context, apiKeyIDStr string) bool {
	apiKeyID, err := uuid.Parse(apiKeyIDStr)
	if err != nil {
		return false
	}

	// Get API key from database (cached)
	apiKeyRepo := s.db.NewAPIKeyRepository()
	apiKey, err := apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return false
	}

	// No budget configured = unlimited
	if apiKey.MonthlyBudgetUSD == nil {
		return true
	}

	budget := *apiKey.MonthlyBudgetUSD

	// Get current month's spending from Redis
	currentSpending, err := s.GetMonthlySpending(ctx, apiKeyIDStr)
	if err != nil {
		// On error, allow request but log
		return true
	}

	return currentSpending < budget
}

// AddUsage adds cost to the running total in Redis
func (s *RedisBillingService) AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error {
	now := time.Now()
	key := s.monthlyKey(apiKeyID, now.Year(), int(now.Month()))

	// Increment cost atomically
	script := redis.NewScript(`
		local key = KEYS[1]
		local cost = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		
		local current = tonumber(redis.call('GET', key)) or 0
		local new_total = current + cost
		
		redis.call('SET', key, new_total, 'EX', ttl)
		return new_total
	`)

	// Keep data for 2 months
	ttl := int((60 * 24 * 60 * 60)) // 60 days in seconds

	_, err := script.Run(ctx, s.redis, []string{key}, costUSD, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to add usage: %w", err)
	}

	return nil
}

// GetMonthlySpending returns the current month's spending for an API key
func (s *RedisBillingService) GetMonthlySpending(ctx context.Context, apiKeyID string) (float64, error) {
	now := time.Now()
	key := s.monthlyKey(apiKeyID, now.Year(), int(now.Month()))

	val, err := s.redis.Get(ctx, key).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get monthly spending: %w", err)
	}

	return val, nil
}

// GetSpending returns spending for a specific month
func (s *RedisBillingService) GetSpending(ctx context.Context, apiKeyID string, year int, month int) (float64, error) {
	key := s.monthlyKey(apiKeyID, year, month)

	val, err := s.redis.Get(ctx, key).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get spending: %w", err)
	}

	return val, nil
}

// ResetMonthlySpending resets spending for current month (admin use)
func (s *RedisBillingService) ResetMonthlySpending(ctx context.Context, apiKeyID string) error {
	now := time.Now()
	key := s.monthlyKey(apiKeyID, now.Year(), int(now.Month()))
	return s.redis.Del(ctx, key).Err()
}

// monthlyKey generates the Redis key for monthly spending
func (s *RedisBillingService) monthlyKey(apiKeyID string, year int, month int) string {
	return fmt.Sprintf("cost:%s:%d:%02d", apiKeyID, year, month)
}

// syncWorker periodically syncs Redis data to PostgreSQL
func (s *RedisBillingService) syncWorker() {
	ticker := time.NewTicker(s.syncFreq)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.syncToDatabase(ctx); err != nil {
			// Log error but continue
			fmt.Printf("Failed to sync billing data: %v\n", err)
		}
		cancel()
	}
}

// syncToDatabase syncs all Redis billing data to PostgreSQL
func (s *RedisBillingService) syncToDatabase(ctx context.Context) error {
	// Scan for all cost keys
	var cursor uint64
	pattern := "cost:*"

	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}

		// Process each key
		for _, key := range keys {
			if err := s.syncKey(ctx, key); err != nil {
				fmt.Printf("Failed to sync key %s: %v\n", key, err)
				// Continue with other keys
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// syncKey syncs a single Redis key to database
func (s *RedisBillingService) syncKey(ctx context.Context, key string) error {
	// Parse key: cost:<api_key_id>:<year>:<month>
	var apiKeyID string
	var year, month int

	_, err := fmt.Sscanf(key, "cost:%s:%d:%d", &apiKeyID, &year, &month)
	if err != nil {
		return fmt.Errorf("invalid key format: %w", err)
	}

	// Get value from Redis
	cost, err := s.redis.Get(ctx, key).Float64()
	if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}

	// Parse UUID
	keyUUID, err := uuid.Parse(apiKeyID)
	if err != nil {
		return fmt.Errorf("invalid API key UUID: %w", err)
	}

	// Note: We don't have direct access to monthly summary repository here
	// This would need to be refactored to accept it as a dependency
	// For now, this is a placeholder showing the pattern

	// TODO: Update monthly_usage_summary in database
	// summaryRepo := s.db.NewMonthlyUsageSummaryRepository()
	// summary, err := summaryRepo.GetByAPIKeyAndMonth(ctx, keyUUID, year, month)
	// if err != nil {
	// 	 // Create new summary
	// } else {
	//   // Update existing
	// }

	_ = keyUUID // Suppress unused warning

	return nil
}

// Shutdown gracefully shuts down the billing service
func (s *RedisBillingService) Shutdown(ctx context.Context) error {
	// Final sync before shutdown
	return s.syncToDatabase(ctx)
}
