package billing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

var billingLogger = utils.NewLogger("billing-service", utils.Info)

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
	summary  monthlySummaryStore

	syncSem    chan struct{}
	syncRunner func(context.Context) error

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

type monthlySummaryStore interface {
	GetByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) (*models.MonthlyUsageSummary, error)
	UpsertCost(ctx context.Context, apiKeyID uuid.UUID, year, month int, totalCostUSD float64) error
	DeleteByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) error
}

var errSyncAlreadyRunning = errors.New("billing sync already in progress")

// NewRedisBillingService creates a new billing service
func NewRedisBillingService(redis *redis.Client, db *storage.DB, syncFrequency time.Duration) *RedisBillingService {
	syncSem := make(chan struct{}, 1)
	syncSem <- struct{}{}

	service := &RedisBillingService{
		redis:    redis,
		db:       db,
		syncFreq: syncFrequency,
		syncSem:  syncSem,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	if db != nil {
		service.summary = storage.NewMonthlyUsageSummaryRepository(db)
	}
	service.syncRunner = service.syncToDatabase

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
	year := now.Year()
	month := int(now.Month())
	key := s.monthlyKey(apiKeyID, year, month)

	val, err := s.redis.Get(ctx, key).Float64()
	if err == redis.Nil {
		return s.getPersistedMonthlySpending(ctx, apiKeyID, year, month)
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
	year := now.Year()
	month := int(now.Month())
	key := s.monthlyKey(apiKeyID, year, month)
	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return err
	}

	if s.summary == nil {
		return nil
	}

	apiKeyUUID, err := uuid.Parse(apiKeyID)
	if err != nil {
		return fmt.Errorf("invalid API key UUID: %w", err)
	}

	if err := s.summary.DeleteByAPIKeyAndMonth(ctx, apiKeyUUID, year, month); err != nil {
		return fmt.Errorf("failed to reset persisted monthly spending: %w", err)
	}

	return nil
}

// monthlyKey generates the Redis key for monthly spending
func (s *RedisBillingService) monthlyKey(apiKeyID string, year int, month int) string {
	return fmt.Sprintf("cost:%s:%d:%02d", apiKeyID, year, month)
}

// syncWorker periodically syncs Redis data to PostgreSQL
func (s *RedisBillingService) syncWorker() {
	defer close(s.doneCh)
	ticker := time.NewTicker(s.syncFreq)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := s.syncWithRetry(ctx)
			cancel()

			if err != nil {
				if errors.Is(err, errSyncAlreadyRunning) {
					billingLogger.Warn("billing sync skipped", "reason", "already running")
					continue
				}
				billingLogger.Error("billing sync failed after retries", "error", err)
			}
		}
	}
}

func (s *RedisBillingService) syncWithRetry(ctx context.Context) error {
	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := s.tryRunSync(ctx)
		if err == nil {
			return nil
		}

		if errors.Is(err, errSyncAlreadyRunning) {
			return err
		}

		lastErr = err
		if attempt < maxAttempts {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return fmt.Errorf("billing sync context cancelled during retry: %w", ctx.Err())
			}
		}
	}

	return fmt.Errorf("billing sync failed after %d attempts: %w", maxAttempts, lastErr)
}

func (s *RedisBillingService) tryRunSync(ctx context.Context) error {
	select {
	case <-s.syncSem:
		defer func() {
			s.syncSem <- struct{}{}
		}()
	default:
		return errSyncAlreadyRunning
	}

	return s.syncRunner(ctx)
}

// syncToDatabase syncs all Redis billing data to PostgreSQL
func (s *RedisBillingService) syncToDatabase(ctx context.Context) error {
	// Scan for all cost keys
	var cursor uint64
	pattern := "cost:*"
	failedKeys := make([]string, 0)

	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}

		// Process each key
		for _, key := range keys {
			if err := s.syncKey(ctx, key); err != nil {
				billingLogger.Warn("failed to sync billing key", "key", key, "error", err)
				failedKeys = append(failedKeys, key)
				// Continue with other keys
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(failedKeys) > 0 {
		return fmt.Errorf("failed to sync %d billing key(s): %s", len(failedKeys), strings.Join(failedKeys, ","))
	}

	return nil
}

// syncKey syncs a single Redis key to database
func (s *RedisBillingService) syncKey(ctx context.Context, key string) error {
	apiKeyID, year, month, err := parseMonthlyCostKey(key)
	if err != nil {
		return err
	}

	// Get value from Redis
	totalCostUSD, err := s.redis.Get(ctx, key).Float64()
	if err != nil {
		return fmt.Errorf("failed to get value: %w", err)
	}

	if s.summary == nil {
		return nil
	}

	return s.summary.UpsertCost(ctx, apiKeyID, year, month, totalCostUSD)
}

func (s *RedisBillingService) getPersistedMonthlySpending(ctx context.Context, apiKeyID string, year, month int) (float64, error) {
	if s.summary == nil {
		return 0, nil
	}

	apiKeyUUID, err := uuid.Parse(apiKeyID)
	if err != nil {
		return 0, fmt.Errorf("invalid API key UUID: %w", err)
	}

	summary, err := s.summary.GetByAPIKeyAndMonth(ctx, apiKeyUUID, year, month)
	if err != nil {
		if errors.Is(err, storage.ErrMonthlyUsageSummaryNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get persisted monthly spending: %w", err)
	}

	totalCostUSD := summary.TotalCostUSD
	if err := s.setMonthlySpending(ctx, apiKeyID, year, month, totalCostUSD); err != nil {
		billingLogger.Warn("failed to repopulate monthly spending cache", "api_key_id", apiKeyID, "year", year, "month", month, "error", err)
	}

	return totalCostUSD, nil
}

func (s *RedisBillingService) setMonthlySpending(ctx context.Context, apiKeyID string, year, month int, totalCostUSD float64) error {
	key := s.monthlyKey(apiKeyID, year, month)
	if err := s.redis.Set(ctx, key, totalCostUSD, 60*24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set monthly spending: %w", err)
	}
	return nil
}

func parseMonthlyCostKey(key string) (uuid.UUID, int, int, error) {
	parts := strings.Split(key, ":")
	if len(parts) != 4 || parts[0] != "cost" {
		return uuid.Nil, 0, 0, fmt.Errorf("invalid key format: %s", key)
	}

	apiKeyID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, 0, 0, fmt.Errorf("invalid API key UUID: %w", err)
	}

	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return uuid.Nil, 0, 0, fmt.Errorf("invalid billing year: %w", err)
	}

	month, err := strconv.Atoi(parts[3])
	if err != nil {
		return uuid.Nil, 0, 0, fmt.Errorf("invalid billing month: %w", err)
	}
	if month < 1 || month > 12 {
		return uuid.Nil, 0, 0, fmt.Errorf("invalid billing month: %d", month)
	}

	return apiKeyID, year, month, nil
}

// Shutdown gracefully shuts down the billing service
func (s *RedisBillingService) Shutdown(ctx context.Context) error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})

	select {
	case <-s.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	return s.syncWithRetry(ctx)
}
