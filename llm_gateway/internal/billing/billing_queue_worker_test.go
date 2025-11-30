package billing

import (
	"context"
	"sync"
	"testing"
	"time"

	"llm_gateway/internal/queue"
)

// mockBillingService implements Service for testing
type mockBillingService struct {
	mu           sync.Mutex
	usage        map[string]float64
	withinBudget bool
}

func newMockBillingService() *mockBillingService {
	return &mockBillingService{
		usage:        make(map[string]float64),
		withinBudget: true,
	}
}

func (m *mockBillingService) WithinBudget(ctx context.Context, apiKeyID string) bool {
	return m.withinBudget
}

func (m *mockBillingService) AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usage[apiKeyID] += costUSD
	return nil
}

func (m *mockBillingService) getUsage(apiKeyID string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.usage[apiKeyID]
}

func TestBillingBillingQueueWorker_SingleUpdate(t *testing.T) {
	config := queue.DefaultConfig("test-billing")
	config.BatchSize = 10
	config.BatchTimeout = 100 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()
	service := newMockBillingService()

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Enqueue a billing update
	update := &BillingUpdate{
		APIKeyID:  "test-api-key",
		CostUSD:   10.50,
		Timestamp: time.Now(),
	}

	err := worker.Enqueue(ctx, update)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify usage was added
	usage := service.getUsage("test-api-key")
	if usage != 10.50 {
		t.Errorf("Expected usage 10.50, got %f", usage)
	}
}

func TestBillingBillingQueueWorker_BatchProcessing(t *testing.T) {
	config := queue.DefaultConfig("test-billing-batch")
	config.BatchSize = 5
	config.BatchTimeout = 100 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()
	service := newMockBillingService()

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Enqueue multiple updates
	for i := 0; i < 10; i++ {
		update := &BillingUpdate{
			APIKeyID:  "test-api-key",
			CostUSD:   1.0,
			Timestamp: time.Now(),
		}
		err := worker.Enqueue(ctx, update)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	// Verify all updates were processed
	usage := service.getUsage("test-api-key")
	if usage != 10.0 {
		t.Errorf("Expected usage 10.0, got %f", usage)
	}
}

func TestBillingBillingQueueWorker_MultipleKeys(t *testing.T) {
	config := queue.DefaultConfig("test-billing-multi")
	config.BatchSize = 10
	config.BatchTimeout = 100 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()
	service := newMockBillingService()

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Enqueue updates for different API keys
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		for i := 0; i < 5; i++ {
			update := &BillingUpdate{
				APIKeyID:  key,
				CostUSD:   2.0,
				Timestamp: time.Now(),
			}
			err := worker.Enqueue(ctx, update)
			if err != nil {
				t.Fatalf("Enqueue failed: %v", err)
			}
		}
	}

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	// Verify each key has correct usage
	for _, key := range keys {
		usage := service.getUsage(key)
		if usage != 10.0 {
			t.Errorf("Expected usage 10.0 for %s, got %f", key, usage)
		}
	}
}

func TestBillingBillingQueueWorker_QueueLength(t *testing.T) {
	config := queue.DefaultConfig("test-billing-length")
	config.BatchSize = 100
	config.BatchTimeout = 1 * time.Second // Long timeout to keep items in queue

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()
	service := newMockBillingService()

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx := context.Background()

	// Don't start worker yet - we want to check queue length

	// Enqueue updates
	for i := 0; i < 10; i++ {
		update := &BillingUpdate{
			APIKeyID:  "test-api-key",
			CostUSD:   1.0,
			Timestamp: time.Now(),
		}
		err := worker.Enqueue(ctx, update)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Check queue length
	length, err := worker.GetQueueLength(ctx)
	if err != nil {
		t.Fatalf("GetQueueLength failed: %v", err)
	}

	if length != 10 {
		t.Errorf("Expected queue length 10, got %d", length)
	}
}

func TestBillingBillingQueueWorker_GracefulShutdown(t *testing.T) {
	config := queue.DefaultConfig("test-billing-shutdown")
	config.BatchSize = 10
	config.BatchTimeout = 50 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()
	service := newMockBillingService()

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)

	// Enqueue updates
	for i := 0; i < 5; i++ {
		update := &BillingUpdate{
			APIKeyID:  "test-api-key",
			CostUSD:   1.0,
			Timestamp: time.Now(),
		}
		err := worker.Enqueue(ctx, update)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Stop worker
	err := worker.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Worker should have stopped gracefully
	// Note: In a real scenario, we'd want to ensure pending items are processed
	// before shutdown, but for this test we just verify it doesn't hang
}

func TestBillingBillingQueueWorker_ConcurrentEnqueue(t *testing.T) {
	config := queue.DefaultConfig("test-billing-concurrent")
	config.BatchSize = 50
	config.BatchTimeout = 100 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()
	service := newMockBillingService()

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Concurrent enqueue from multiple goroutines
	var wg sync.WaitGroup
	numGoroutines := 10
	updatesPerGoroutine := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < updatesPerGoroutine; j++ {
				update := &BillingUpdate{
					APIKeyID:  "test-api-key",
					CostUSD:   0.5,
					Timestamp: time.Now(),
				}
				_ = worker.Enqueue(ctx, update)
			}
		}()
	}

	wg.Wait()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify total usage
	expectedTotal := float64(numGoroutines*updatesPerGoroutine) * 0.5
	usage := service.getUsage("test-api-key")
	if usage != expectedTotal {
		t.Errorf("Expected usage %f, got %f", expectedTotal, usage)
	}
}

// mockFailingBillingService simulates failures
type mockFailingBillingService struct {
	mu        sync.Mutex
	failCount int
	maxFails  int
	usage     map[string]float64
}

func newMockFailingBillingService(maxFails int) *mockFailingBillingService {
	return &mockFailingBillingService{
		maxFails: maxFails,
		usage:    make(map[string]float64),
	}
}

func (m *mockFailingBillingService) WithinBudget(ctx context.Context, apiKeyID string) bool {
	return true
}

func (m *mockFailingBillingService) AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failCount++
	if m.failCount <= m.maxFails {
		return queue.ErrMaxRetriesExceeded // Simulate failure
	}

	m.usage[apiKeyID] += costUSD
	return nil
}

func (m *mockFailingBillingService) getUsage(apiKeyID string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.usage[apiKeyID]
}

func TestBillingQueueWorker_RetryOnFailure(t *testing.T) {
	config := queue.DefaultConfig("test-billing-retry")
	config.BatchSize = 10
	config.BatchTimeout = 50 * time.Millisecond
	config.MaxRetries = 3
	config.RetryBackoff = 10 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()

	// Service that fails twice then succeeds
	service := newMockFailingBillingService(2)

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Enqueue update
	update := &BillingUpdate{
		APIKeyID:  "test-api-key",
		CostUSD:   5.0,
		Timestamp: time.Now(),
	}

	err := worker.Enqueue(ctx, update)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Wait for retries and processing
	time.Sleep(500 * time.Millisecond)

	// Should eventually succeed after retries
	usage := service.getUsage("test-api-key")
	if usage != 5.0 {
		t.Errorf("Expected usage 5.0 after retries, got %f", usage)
	}
}

func TestBillingQueueWorker_DeadLetterQueue(t *testing.T) {
	config := queue.DefaultConfig("test-billing-dlq")
	config.BatchSize = 10
	config.BatchTimeout = 50 * time.Millisecond
	config.MaxRetries = 2
	config.RetryBackoff = 10 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()

	// Service that always fails
	service := newMockFailingBillingService(100)

	worker := NewBillingQueueWorker(q, dlq, service, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	defer worker.Stop()

	// Enqueue update
	update := &BillingUpdate{
		APIKeyID:  "test-api-key",
		CostUSD:   5.0,
		Timestamp: time.Now(),
	}

	err := worker.Enqueue(ctx, update)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Wait for retries to exhaust
	time.Sleep(500 * time.Millisecond)

	// Should be in DLQ after max retries
	dlqItems, err := worker.GetDeadLetterItems(ctx, 10)
	if err != nil {
		t.Fatalf("GetDeadLetterItems failed: %v", err)
	}

	if len(dlqItems) != 1 {
		t.Errorf("Expected 1 item in DLQ, got %d", len(dlqItems))
	}
}
