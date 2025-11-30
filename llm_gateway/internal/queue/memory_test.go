package queue

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryQueue_EnqueueDequeue(t *testing.T) {
	config := DefaultConfig("test")
	q := NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Test single item
	item := "test-item-1"
	err := q.Enqueue(ctx, item)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	items, err := q.Dequeue(ctx, 1)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	if items[0].(string) != item {
		t.Errorf("Expected %s, got %v", item, items[0])
	}
}

func TestMemoryQueue_MultipleBatch(t *testing.T) {
	config := DefaultConfig("test")
	config.BatchSize = 5
	q := NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Enqueue multiple items
	for i := 0; i < 10; i++ {
		err := q.Enqueue(ctx, i)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Dequeue in batches
	items, err := q.Dequeue(ctx, 5)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items, got %d", len(items))
	}

	// Dequeue remaining
	items, err = q.Dequeue(ctx, 10)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items, got %d", len(items))
	}
}

func TestMemoryQueue_DequeueWithTimeout(t *testing.T) {
	config := DefaultConfig("test")
	q := NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Test timeout with no items
	start := time.Now()
	items, err := q.DequeueWithTimeout(ctx, 1, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DequeueWithTimeout failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected timeout, but returned early: %v", elapsed)
	}

	// Test with items available
	err = q.Enqueue(ctx, "test")
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	items, err = q.DequeueWithTimeout(ctx, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("DequeueWithTimeout failed: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
}

func TestMemoryQueue_Length(t *testing.T) {
	config := DefaultConfig("test")
	q := NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()

	// Initial length should be 0
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 0 {
		t.Errorf("Expected length 0, got %d", length)
	}

	// Add items
	for i := 0; i < 5; i++ {
		err := q.Enqueue(ctx, i)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	length, err = q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 5 {
		t.Errorf("Expected length 5, got %d", length)
	}
}

func TestMemoryQueue_Concurrent(t *testing.T) {
	config := DefaultConfig("test")
	q := NewMemoryQueue(config)
	defer q.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent enqueue
	numGoroutines := 10
	itemsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				err := q.Enqueue(ctx, id*itemsPerGoroutine+j)
				if err != nil {
					t.Errorf("Enqueue failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all items were enqueued
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}

	expected := numGoroutines * itemsPerGoroutine
	if length != expected {
		t.Errorf("Expected length %d, got %d", expected, length)
	}
}

func TestMemoryQueue_ClosedQueue(t *testing.T) {
	config := DefaultConfig("test")
	q := NewMemoryQueue(config)

	ctx := context.Background()

	// Close queue
	err := q.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations on closed queue should fail
	err = q.Enqueue(ctx, "test")
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}

	_, err = q.Length(ctx)
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}
}

func TestMemoryDeadLetterQueue_AddList(t *testing.T) {
	dlq := NewMemoryDeadLetterQueue()
	defer dlq.Close()

	ctx := context.Background()

	// Add items
	item1 := "test-item-1"
	err1 := dlq.Add(ctx, item1, ErrMaxRetriesExceeded)
	if err1 != nil {
		t.Fatalf("Add failed: %v", err1)
	}

	item2 := "test-item-2"
	err2 := dlq.Add(ctx, item2, ErrQueueClosed)
	if err2 != nil {
		t.Fatalf("Add failed: %v", err2)
	}

	// List items
	items, err := dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// Verify error messages
	for _, item := range items {
		if item.Error == "" {
			t.Error("Expected non-empty error message")
		}
		if item.ID == "" {
			t.Error("Expected non-empty ID")
		}
	}
}

func TestMemoryDeadLetterQueue_Remove(t *testing.T) {
	dlq := NewMemoryDeadLetterQueue()
	defer dlq.Close()

	ctx := context.Background()

	// Add item
	item := "test-item"
	err := dlq.Add(ctx, item, ErrMaxRetriesExceeded)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// List to get ID
	items, err := dlq.List(ctx, 1)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	itemID := items[0].ID

	// Remove item
	err = dlq.Remove(ctx, itemID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify removed
	items, err = dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items after removal, got %d", len(items))
	}
}

func TestMemoryDeadLetterQueue_RemoveNonExistent(t *testing.T) {
	dlq := NewMemoryDeadLetterQueue()
	defer dlq.Close()

	ctx := context.Background()

	// Try to remove non-existent item
	err := dlq.Remove(ctx, "non-existent-id")
	if err != ErrItemNotFound {
		t.Errorf("Expected ErrItemNotFound, got %v", err)
	}
}

func TestMemoryDeadLetterQueue_Closed(t *testing.T) {
	dlq := NewMemoryDeadLetterQueue()

	ctx := context.Background()

	// Close DLQ
	err := dlq.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations on closed DLQ should fail
	err = dlq.Add(ctx, "test", ErrMaxRetriesExceeded)
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}

	_, err = dlq.List(ctx, 10)
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}

	err = dlq.Remove(ctx, "test-id")
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}
}
