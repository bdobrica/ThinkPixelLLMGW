package queue

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// MemoryQueue implements Queue using in-memory channels
type MemoryQueue struct {
	items  chan interface{}
	mu     sync.RWMutex
	closed bool
	config *Config
}

// NewMemoryQueue creates a new in-memory queue
func NewMemoryQueue(config *Config) *MemoryQueue {
	if config == nil {
		config = DefaultConfig("memory")
	}

	return &MemoryQueue{
		items:  make(chan interface{}, config.BatchSize*10), // Buffer for 10 batches
		config: config,
	}
}

// Enqueue adds an item to the queue
func (q *MemoryQueue) Enqueue(ctx context.Context, item interface{}) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return ErrQueueClosed
	}

	select {
	case q.items <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dequeue retrieves items from the queue
func (q *MemoryQueue) Dequeue(ctx context.Context, maxItems int) ([]interface{}, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return nil, ErrQueueClosed
	}

	var items []interface{}

	// Block until we get at least one item
	select {
	case item := <-q.items:
		items = append(items, item)
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Try to get more items without blocking
	for len(items) < maxItems {
		select {
		case item := <-q.items:
			items = append(items, item)
		default:
			return items, nil
		}
	}

	return items, nil
}

// DequeueWithTimeout retrieves items with a timeout
func (q *MemoryQueue) DequeueWithTimeout(ctx context.Context, maxItems int, timeout time.Duration) ([]interface{}, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return nil, ErrQueueClosed
	}

	var items []interface{}
	deadline := time.After(timeout)

	// Try to get first item with timeout
	select {
	case item := <-q.items:
		items = append(items, item)
	case <-deadline:
		return items, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Try to get more items without blocking
	for len(items) < maxItems {
		select {
		case item := <-q.items:
			items = append(items, item)
		default:
			return items, nil
		}
	}

	return items, nil
}

// Length returns the current queue length
func (q *MemoryQueue) Length(ctx context.Context) (int, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return 0, ErrQueueClosed
	}

	return len(q.items), nil
}

// Close shuts down the queue
func (q *MemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil
	}

	q.closed = true
	close(q.items)
	return nil
}

// MemoryDeadLetterQueue implements DeadLetterQueue using in-memory storage
type MemoryDeadLetterQueue struct {
	items  []DeadLetterItem
	mu     sync.RWMutex
	closed bool
}

// NewMemoryDeadLetterQueue creates a new in-memory dead letter queue
func NewMemoryDeadLetterQueue() *MemoryDeadLetterQueue {
	return &MemoryDeadLetterQueue{
		items: make([]DeadLetterItem, 0),
	}
}

// Add adds a failed item to the dead letter queue
func (q *MemoryDeadLetterQueue) Add(ctx context.Context, item interface{}, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	dlItem := DeadLetterItem{
		ID:        generateID(),
		Item:      item,
		Error:     err.Error(),
		Timestamp: time.Now(),
		Retries:   0,
	}

	q.items = append(q.items, dlItem)
	return nil
}

// List retrieves items from the dead letter queue
func (q *MemoryDeadLetterQueue) List(ctx context.Context, maxItems int) ([]DeadLetterItem, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return nil, ErrQueueClosed
	}

	if maxItems <= 0 || maxItems > len(q.items) {
		maxItems = len(q.items)
	}

	result := make([]DeadLetterItem, maxItems)
	copy(result, q.items[:maxItems])
	return result, nil
}

// Remove removes an item from the dead letter queue
func (q *MemoryDeadLetterQueue) Remove(ctx context.Context, id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	for i, item := range q.items {
		if item.ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return nil
		}
	}

	return ErrItemNotFound
}

// Close shuts down the dead letter queue
func (q *MemoryDeadLetterQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	q.items = nil
	return nil
}

// generateID generates a unique ID for dead letter items
func generateID() string {
	return time.Now().Format("20060102150405.000000")
}

// Helper function to serialize items for storage
func serializeItem(item interface{}) ([]byte, error) {
	return json.Marshal(item)
}

// Helper function to deserialize items from storage
func deserializeItem(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
