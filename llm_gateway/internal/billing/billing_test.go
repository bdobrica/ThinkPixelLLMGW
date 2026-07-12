package billing

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
)

type mockMonthlySummaryStore struct {
	summaries map[string]float64
	deleted   map[string]bool
	upserts   int
}

func newMockMonthlySummaryStore() *mockMonthlySummaryStore {
	return &mockMonthlySummaryStore{
		summaries: make(map[string]float64),
		deleted:   make(map[string]bool),
	}
}

func (m *mockMonthlySummaryStore) GetByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) (*models.MonthlyUsageSummary, error) {
	key := summaryKey(apiKeyID, year, month)
	totalCostUSD, ok := m.summaries[key]
	if !ok {
		return nil, storage.ErrMonthlyUsageSummaryNotFound
	}
	return &models.MonthlyUsageSummary{TotalCostUSD: totalCostUSD}, nil
}

func (m *mockMonthlySummaryStore) UpsertCost(ctx context.Context, apiKeyID uuid.UUID, year, month int, totalCostUSD float64) error {
	m.summaries[summaryKey(apiKeyID, year, month)] = totalCostUSD
	m.upserts++
	return nil
}

func (m *mockMonthlySummaryStore) DeleteByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) error {
	key := summaryKey(apiKeyID, year, month)
	delete(m.summaries, key)
	m.deleted[key] = true
	return nil
}

func summaryKey(apiKeyID uuid.UUID, year, month int) string {
	return fmt.Sprintf("%s:%d:%02d", apiKeyID.String(), year, month)
}

func TestNoopService_WithinBudget(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	testCases := []string{
		"api-key-1",
		"api-key-2",
		"",
		"invalid-uuid",
	}

	for _, apiKeyID := range testCases {
		t.Run(apiKeyID, func(t *testing.T) {
			result := service.WithinBudget(ctx, apiKeyID)
			if !result {
				t.Errorf("NoopService.WithinBudget() = false, want true")
			}
		})
	}
}

func TestNoopService_AddUsage(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	testCases := []struct {
		name     string
		apiKeyID string
		cost     float64
	}{
		{
			name:     "positive cost",
			apiKeyID: "api-key-1",
			cost:     10.50,
		},
		{
			name:     "zero cost",
			apiKeyID: "api-key-2",
			cost:     0.0,
		},
		{
			name:     "large cost",
			apiKeyID: "api-key-3",
			cost:     1000.99,
		},
		{
			name:     "small cost",
			apiKeyID: "api-key-4",
			cost:     0.001,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.AddUsage(ctx, tc.apiKeyID, tc.cost)
			if err != nil {
				t.Errorf("NoopService.AddUsage() error = %v, want nil", err)
			}
		})
	}
}

func TestNoopService_Multiple(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	// Add multiple usage entries
	apiKeyID := "test-key"
	costs := []float64{1.0, 2.5, 3.75, 10.0}

	for _, cost := range costs {
		if err := service.AddUsage(ctx, apiKeyID, cost); err != nil {
			t.Errorf("NoopService.AddUsage() error = %v", err)
		}
	}

	// Check budget (should always be within budget)
	if !service.WithinBudget(ctx, apiKeyID) {
		t.Error("NoopService.WithinBudget() = false after adding usage, want true")
	}
}

func TestNoopService_Concurrent(t *testing.T) {
	service := NewNoopService()
	ctx := context.Background()

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			apiKeyID := "concurrent-key"
			service.AddUsage(ctx, apiKeyID, 1.0)
			service.WithinBudget(ctx, apiKeyID)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRedisBillingService_TryRunSync_PreventsOverlap(t *testing.T) {
	service := &RedisBillingService{
		syncSem: make(chan struct{}, 1),
	}
	service.syncSem <- struct{}{}

	release := make(chan struct{})
	service.syncRunner = func(ctx context.Context) error {
		<-release
		return nil
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- service.tryRunSync(context.Background())
	}()

	time.Sleep(30 * time.Millisecond)
	err := service.tryRunSync(context.Background())
	if !errors.Is(err, errSyncAlreadyRunning) {
		t.Fatalf("expected errSyncAlreadyRunning, got: %v", err)
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first sync returned unexpected error: %v", err)
	}
}

func TestRedisBillingService_SyncWithRetry_RetriesThenSucceeds(t *testing.T) {
	service := &RedisBillingService{
		syncSem: make(chan struct{}, 1),
	}
	service.syncSem <- struct{}{}

	var attempts atomic.Int32
	service.syncRunner = func(ctx context.Context) error {
		n := attempts.Add(1)
		if n < 3 {
			return fmt.Errorf("transient error")
		}
		return nil
	}

	err := service.syncWithRetry(context.Background())
	if err != nil {
		t.Fatalf("syncWithRetry returned error: %v", err)
	}

	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRedisBillingService_SyncToDatabase_ReturnsErrorWhenAnyKeyFails(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	ctx := context.Background()
	validAPIKeyID := uuid.New().String()
	if err := client.Set(ctx, fmt.Sprintf("cost:%s:%d:%02d", validAPIKeyID, 2026, 3), "1.25", 0).Err(); err != nil {
		t.Fatalf("failed to seed valid key: %v", err)
	}
	if err := client.Set(ctx, "cost:not-a-uuid:2026:03", "2.0", 0).Err(); err != nil {
		t.Fatalf("failed to seed invalid key: %v", err)
	}

	service := &RedisBillingService{
		redis:   client,
		summary: newMockMonthlySummaryStore(),
	}

	err = service.syncToDatabase(ctx)
	if err == nil {
		t.Fatal("expected syncToDatabase to return error when a key fails")
	}
}

func TestParseMonthlyCostKey(t *testing.T) {
	apiKeyID := uuid.New()

	testCases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "valid key", key: fmt.Sprintf("cost:%s:2026:03", apiKeyID)},
		{name: "invalid prefix", key: fmt.Sprintf("billing:%s:2026:03", apiKeyID), wantErr: true},
		{name: "invalid uuid", key: "cost:not-a-uuid:2026:03", wantErr: true},
		{name: "invalid month", key: fmt.Sprintf("cost:%s:2026:13", apiKeyID), wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := parseMonthlyCostKey(tc.key)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRedisBillingService_GetMonthlySpending_FallsBackToPersistedSummary(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	apiKeyID := uuid.New()
	now := time.Now()
	summaryStore := newMockMonthlySummaryStore()
	summaryStore.summaries[summaryKey(apiKeyID, now.Year(), int(now.Month()))] = 12.34

	service := &RedisBillingService{
		redis:   client,
		summary: summaryStore,
	}

	spending, err := service.GetMonthlySpending(context.Background(), apiKeyID.String())
	if err != nil {
		t.Fatalf("GetMonthlySpending returned error: %v", err)
	}
	if spending != 12.34 {
		t.Fatalf("GetMonthlySpending = %v, want 12.34", spending)
	}

	redisValue, err := client.Get(context.Background(), service.monthlyKey(apiKeyID.String(), now.Year(), int(now.Month()))).Int64()
	if err != nil {
		t.Fatalf("expected Redis cache to be repopulated: %v", err)
	}
	if redisValue != 12_340_000_000 {
		t.Fatalf("redis cached value = %v, want 12340000000 nano-USD", redisValue)
	}
}

func TestRedisBillingService_AccumulatesNanoUSDWithoutDrift(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	service := &RedisBillingService{redis: client}
	apiKeyID := uuid.New().String()
	for i := 0; i < 10_000; i++ {
		if err := service.AddUsage(context.Background(), apiKeyID, 0.000000001); err != nil {
			t.Fatal(err)
		}
	}
	got, err := service.GetMonthlySpending(context.Background(), apiKeyID)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.00001 {
		t.Fatalf("got %.12f, want %.12f", got, 0.00001)
	}
}

func TestRedisBillingService_ResetMonthlySpending_ClearsPersistedSummary(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	apiKeyID := uuid.New()
	now := time.Now()
	summaryStore := newMockMonthlySummaryStore()
	key := summaryKey(apiKeyID, now.Year(), int(now.Month()))
	summaryStore.summaries[key] = 9.99

	service := &RedisBillingService{
		redis:   client,
		summary: summaryStore,
	}

	if err := client.Set(context.Background(), service.monthlyKey(apiKeyID.String(), now.Year(), int(now.Month())), 9.99, 0).Err(); err != nil {
		t.Fatalf("failed to seed redis spending: %v", err)
	}

	if err := service.ResetMonthlySpending(context.Background(), apiKeyID.String()); err != nil {
		t.Fatalf("ResetMonthlySpending returned error: %v", err)
	}

	if _, err := client.Get(context.Background(), service.monthlyKey(apiKeyID.String(), now.Year(), int(now.Month()))).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected Redis spending to be cleared, got err=%v", err)
	}
	if _, exists := summaryStore.summaries[key]; exists {
		t.Fatal("expected persisted summary to be removed")
	}
	if !summaryStore.deleted[key] {
		t.Fatal("expected persisted summary delete to be recorded")
	}
}
