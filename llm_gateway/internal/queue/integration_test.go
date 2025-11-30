package queue

import (
	"context"
	"testing"
	"time"
)

// TestQueueIntegration tests the complete flow of enqueuing, processing, and DLQ
func TestQueueIntegration(t *testing.T) {
	config := DefaultConfig("integration-test")
	config.BatchSize = 5
	config.BatchTimeout = 100 * time.Millisecond

	q := NewMemoryQueue(config)
	dlq := NewMemoryDeadLetterQueue()
	defer q.Close()
	defer dlq.Close()

	ctx := context.Background()

	// Enqueue items
	for i := 0; i < 10; i++ {
		err := q.Enqueue(ctx, map[string]int{"id": i})
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

	// Process first batch
	items, err := q.Dequeue(ctx, config.BatchSize)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("Expected 5 items in batch, got %d", len(items))
	}

	// Simulate processing failure - add to DLQ
	failedItem := items[0]
	err = dlq.Add(ctx, failedItem, ErrMaxRetriesExceeded)
	if err != nil {
		t.Fatalf("DLQ Add failed: %v", err)
	}

	// Process second batch
	items, err = q.Dequeue(ctx, config.BatchSize)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("Expected 5 items in second batch, got %d", len(items))
	}

	// Queue should now be empty
	length, err = q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected queue to be empty, got length %d", length)
	}

	// DLQ should have 1 item
	dlqItems, err := dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("DLQ List failed: %v", err)
	}
	if len(dlqItems) != 1 {
		t.Errorf("Expected 1 item in DLQ, got %d", len(dlqItems))
	}

	// Verify DLQ item details
	if dlqItems[0].Error != ErrMaxRetriesExceeded.Error() {
		t.Errorf("Expected error %v, got %s", ErrMaxRetriesExceeded, dlqItems[0].Error)
	}

	// Retry DLQ item - re-enqueue it
	err = q.Enqueue(ctx, dlqItems[0].Item)
	if err != nil {
		t.Fatalf("Re-enqueue failed: %v", err)
	}

	// Remove from DLQ
	err = dlq.Remove(ctx, dlqItems[0].ID)
	if err != nil {
		t.Fatalf("DLQ Remove failed: %v", err)
	}

	// Verify DLQ is now empty
	dlqItems, err = dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("DLQ List failed: %v", err)
	}
	if len(dlqItems) != 0 {
		t.Errorf("Expected DLQ to be empty, got %d items", len(dlqItems))
	}

	// Process retried item
	items, err = q.Dequeue(ctx, 1)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Expected 1 retried item, got %d", len(items))
	}
}

// TestBatchProcessing tests batch processing behavior
func TestBatchProcessing(t *testing.T) {
	config := DefaultConfig("batch-test")
	config.BatchSize = 10
	config.BatchTimeout = 200 * time.Millisecond

	q := NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Enqueue 5 items (less than batch size)
	for i := 0; i < 5; i++ {
		err := q.Enqueue(ctx, i)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Dequeue with timeout - should return immediately with available items
	start := time.Now()
	items, err := q.DequeueWithTimeout(ctx, config.BatchSize, config.BatchTimeout)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DequeueWithTimeout failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items (partial batch), got %d", len(items))
	}

	// Should return quickly since items are available
	if elapsed > 50*time.Millisecond {
		t.Errorf("Expected quick return with available items, but took %v", elapsed)
	}

	// Enqueue exactly batch size items
	for i := 0; i < 10; i++ {
		err := q.Enqueue(ctx, i)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Dequeue should return immediately with full batch
	start = time.Now()
	items, err = q.Dequeue(ctx, config.BatchSize)
	elapsed = time.Since(start)

	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 10 {
		t.Errorf("Expected 10 items (full batch), got %d", len(items))
	}

	// Should return quickly (not wait for timeout)
	if elapsed > 50*time.Millisecond {
		t.Errorf("Expected quick return, but took %v", elapsed)
	}
}

// TestConcurrentProducerConsumer tests concurrent enqueue and dequeue
func TestConcurrentProducerConsumer(t *testing.T) {
	config := DefaultConfig("concurrent-test")
	config.BatchSize = 20
	q := NewMemoryQueue(config)
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	itemsToProcess := 100
	processedCount := 0
	doneChan := make(chan bool)

	// Producer goroutine
	go func() {
		for i := 0; i < itemsToProcess; i++ {
			_ = q.Enqueue(ctx, i)
			time.Sleep(1 * time.Millisecond) // Simulate some delay
		}
	}()

	// Consumer goroutine
	go func() {
		for processedCount < itemsToProcess {
			items, err := q.DequeueWithTimeout(ctx, config.BatchSize, 50*time.Millisecond)
			if err != nil {
				continue
			}
			processedCount += len(items)
		}
		doneChan <- true
	}()

	// Wait for completion with timeout
	select {
	case <-doneChan:
		if processedCount != itemsToProcess {
			t.Errorf("Expected %d items processed, got %d", itemsToProcess, processedCount)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Test timed out - processed %d/%d items", processedCount, itemsToProcess)
	}
}
