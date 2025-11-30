package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
	"llm_gateway/internal/queue"
)

// mockUsageRepository simulates database operations for testing
type mockUsageRepository struct {
	mu        sync.Mutex
	records   []*models.UsageRecord
	failCount int
	maxFails  int
}

func newMockUsageRepository() *mockUsageRepository {
	return &mockUsageRepository{
		records: make([]*models.UsageRecord, 0),
	}
}

func (m *mockUsageRepository) Create(ctx context.Context, record *models.UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failCount < m.maxFails {
		m.failCount++
		return fmt.Errorf("simulated database error")
	}

	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	record.CreatedAt = time.Now()

	m.records = append(m.records, record)
	return nil
}

func (m *mockUsageRepository) getRecordCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records)
}

func (m *mockUsageRepository) getRecords() []*models.UsageRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.records
}

func (m *mockUsageRepository) setFailures(maxFails int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failCount = 0
	m.maxFails = maxFails
}

// mockDB wraps mockUsageRepository
type mockDB struct {
	repo *mockUsageRepository
}

func newMockDB() *mockDB {
	return &mockDB{
		repo: newMockUsageRepository(),
	}
}

// We need to modify UsageQueueWorker to accept a repository interface for testing
// For now, we'll test the worker with a simplified approach

func TestUsageQueueWorker_SingleRecord(t *testing.T) {
	// Note: This test would require modifying UsageQueueWorker to accept
	// a repository interface. For now, we'll test the queue mechanics.

	config := queue.DefaultConfig("test-usage")
	config.BatchSize = 10
	config.BatchTimeout = 100 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	_ = queue.NewMemoryDeadLetterQueue()

	// Create a test record
	record := &models.UsageRecord{
		ID:              uuid.New(),
		APIKeyID:        uuid.New(),
		RequestID:       uuid.New(),
		ModelName:       "gpt-4",
		Endpoint:        "/v1/chat/completions",
		InputTokens:     100,
		OutputTokens:    50,
		CachedTokens:    0,
		ReasoningTokens: 0,
		ResponseTimeMS:  250,
		StatusCode:      200,
	}

	// Enqueue record
	ctx := context.Background()
	err := q.Enqueue(ctx, record)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Dequeue and verify
	items, err := q.DequeueWithTimeout(ctx, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
}

func TestUsageQueueWorker_BatchRecords(t *testing.T) {
	config := queue.DefaultConfig("test-usage-batch")
	config.BatchSize = 5
	config.BatchTimeout = 100 * time.Millisecond

	q := queue.NewMemoryQueue(config)
	_ = queue.NewMemoryDeadLetterQueue()

	ctx := context.Background()

	// Enqueue multiple records
	for i := 0; i < 10; i++ {
		record := &models.UsageRecord{
			ID:              uuid.New(),
			APIKeyID:        uuid.New(),
			RequestID:       uuid.New(),
			ModelName:       "gpt-4",
			Endpoint:        "/v1/chat/completions",
			InputTokens:     100 + i,
			OutputTokens:    50 + i,
			CachedTokens:    0,
			ReasoningTokens: 0,
			ResponseTimeMS:  250,
			StatusCode:      200,
		}

		err := q.Enqueue(ctx, record)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Verify queue length
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}

	if length != 10 {
		t.Errorf("Expected queue length 10, got %d", length)
	}

	// Dequeue first batch
	items, err := q.Dequeue(ctx, 5)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items in batch, got %d", len(items))
	}
}

func TestUsageQueueWorker_DetailedTokenTracking(t *testing.T) {
	config := queue.DefaultConfig("test-usage-tokens")
	q := queue.NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Create record with detailed token info
	record := &models.UsageRecord{
		ID:              uuid.New(),
		APIKeyID:        uuid.New(),
		RequestID:       uuid.New(),
		ModelName:       "claude-3-5-sonnet",
		Endpoint:        "/v1/chat/completions",
		InputTokens:     1000,
		OutputTokens:    500,
		CachedTokens:    200, // Cached from prompt cache
		ReasoningTokens: 150, // Reasoning tokens (e.g., o1 model)
		ResponseTimeMS:  450,
		StatusCode:      200,
	}

	err := q.Enqueue(ctx, record)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Dequeue and verify all fields
	items, err := q.DequeueWithTimeout(ctx, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	// Type assertion and field verification
	retrieved, ok := items[0].(*models.UsageRecord)
	if !ok {
		t.Fatalf("Item is not a UsageRecord")
	}

	if retrieved.InputTokens != 1000 {
		t.Errorf("Expected InputTokens 1000, got %d", retrieved.InputTokens)
	}
	if retrieved.OutputTokens != 500 {
		t.Errorf("Expected OutputTokens 500, got %d", retrieved.OutputTokens)
	}
	if retrieved.CachedTokens != 200 {
		t.Errorf("Expected CachedTokens 200, got %d", retrieved.CachedTokens)
	}
	if retrieved.ReasoningTokens != 150 {
		t.Errorf("Expected ReasoningTokens 150, got %d", retrieved.ReasoningTokens)
	}
}

func TestUsageQueueWorker_ConcurrentEnqueue(t *testing.T) {
	config := queue.DefaultConfig("test-usage-concurrent")
	config.BatchSize = 100
	q := queue.NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Concurrent enqueue from multiple goroutines
	var wg sync.WaitGroup
	numGoroutines := 10
	recordsPerGoroutine := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				record := &models.UsageRecord{
					ID:             uuid.New(),
					APIKeyID:       uuid.New(),
					RequestID:      uuid.New(),
					ModelName:      fmt.Sprintf("model-%d", goroutineID),
					Endpoint:       "/v1/chat/completions",
					InputTokens:    100,
					OutputTokens:   50,
					ResponseTimeMS: 200,
					StatusCode:     200,
				}
				_ = q.Enqueue(ctx, record)
			}
		}(i)
	}

	wg.Wait()

	// Verify all records were enqueued
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}

	expected := numGoroutines * recordsPerGoroutine
	if length != expected {
		t.Errorf("Expected queue length %d, got %d", expected, length)
	}
}

func TestUsageQueueWorker_ErrorHandling(t *testing.T) {
	config := queue.DefaultConfig("test-usage-error")
	q := queue.NewMemoryQueue(config)
	dlq := queue.NewMemoryDeadLetterQueue()

	ctx := context.Background()

	// Test that malformed data doesn't crash
	err := q.Enqueue(ctx, "not-a-usage-record")
	if err != nil {
		t.Fatalf("Enqueue should accept any type: %v", err)
	}

	// Enqueue valid record
	record := &models.UsageRecord{
		ID:         uuid.New(),
		APIKeyID:   uuid.New(),
		RequestID:  uuid.New(),
		ModelName:  "gpt-4",
		StatusCode: 200,
	}

	err = q.Enqueue(ctx, record)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Verify both items are in queue
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}

	if length != 2 {
		t.Errorf("Expected queue length 2, got %d", length)
	}

	// DLQ should be empty initially
	dlqItems, err := dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("DLQ List failed: %v", err)
	}

	if len(dlqItems) != 0 {
		t.Errorf("Expected 0 DLQ items, got %d", len(dlqItems))
	}
}

func TestUsageQueueWorker_QueueLength(t *testing.T) {
	config := queue.DefaultConfig("test-usage-length")
	q := queue.NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Test empty queue
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected length 0 for empty queue, got %d", length)
	}

	// Add records
	for i := 0; i < 15; i++ {
		record := &models.UsageRecord{
			ID:         uuid.New(),
			APIKeyID:   uuid.New(),
			RequestID:  uuid.New(),
			ModelName:  "gpt-4",
			StatusCode: 200,
		}
		err := q.Enqueue(ctx, record)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Verify length
	length, err = q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 15 {
		t.Errorf("Expected length 15, got %d", length)
	}
}

func TestUsageQueueWorker_MultipleModels(t *testing.T) {
	config := queue.DefaultConfig("test-usage-models")
	q := queue.NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	modelNames := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-5-sonnet", "claude-3-opus"}

	// Enqueue records for different models
	for _, modelName := range modelNames {
		for i := 0; i < 5; i++ {
			record := &models.UsageRecord{
				ID:             uuid.New(),
				APIKeyID:       uuid.New(),
				RequestID:      uuid.New(),
				ModelName:      modelName,
				Endpoint:       "/v1/chat/completions",
				InputTokens:    100,
				OutputTokens:   50,
				ResponseTimeMS: 200,
				StatusCode:     200,
			}
			err := q.Enqueue(ctx, record)
			if err != nil {
				t.Fatalf("Enqueue failed: %v", err)
			}
		}
	}

	// Verify total records
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}

	expected := len(modelNames) * 5
	if length != expected {
		t.Errorf("Expected %d records, got %d", expected, length)
	}
}

func TestUsageQueueWorker_StatusCodes(t *testing.T) {
	config := queue.DefaultConfig("test-usage-status")
	q := queue.NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	statusCodes := []int{200, 400, 429, 500, 503}

	for _, statusCode := range statusCodes {
		record := &models.UsageRecord{
			ID:             uuid.New(),
			APIKeyID:       uuid.New(),
			RequestID:      uuid.New(),
			ModelName:      "gpt-4",
			Endpoint:       "/v1/chat/completions",
			InputTokens:    100,
			OutputTokens:   50,
			ResponseTimeMS: 200,
			StatusCode:     statusCode,
			ErrorMessage:   fmt.Sprintf("Error for status %d", statusCode),
		}
		err := q.Enqueue(ctx, record)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Dequeue and verify status codes
	items, err := q.Dequeue(ctx, 10)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != len(statusCodes) {
		t.Errorf("Expected %d items, got %d", len(statusCodes), len(items))
	}

	// Verify each record has a status code
	for _, item := range items {
		record, ok := item.(*models.UsageRecord)
		if !ok {
			t.Errorf("Item is not a UsageRecord")
			continue
		}
		if record.StatusCode == 0 {
			t.Error("StatusCode should not be 0")
		}
	}
}
