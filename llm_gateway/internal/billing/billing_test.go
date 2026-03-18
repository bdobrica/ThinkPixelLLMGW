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
)

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
		redis: client,
	}

	err = service.syncToDatabase(ctx)
	if err == nil {
		t.Fatal("expected syncToDatabase to return error when a key fails")
	}
}
