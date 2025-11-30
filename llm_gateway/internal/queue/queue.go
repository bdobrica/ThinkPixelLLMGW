package queue

import (
	"context"
	"time"
)

// Package queue provides a hybrid queue system for async processing with two backends:
//
// 1. Memory Queue (in-memory, channel-based):
//    - No persistence, data lost on restart
//    - Zero external dependencies
//    - Perfect for standalone/development deployments (e.g., Raspberry Pi)
//
// 2. Redis Queue (Redis List-based):
//    - Persistent across restarts
//    - Supports distributed workers
//    - Production-ready for Kubernetes deployments
//
// Architecture:
//
//	┌─────────────┐
//	│   Request   │
//	└──────┬──────┘
//	       │
//	       ├─────────────────────────┐
//	       │                         │
//	       ▼                         ▼
//	┌──────────────┐         ┌──────────────┐
//	│ Billing      │         │ Usage        │
//	│ Queue        │         │ Queue        │
//	└──────┬───────┘         └──────┬───────┘
//	       │                         │
//	       ▼                         ▼
//	┌──────────────┐         ┌──────────────┐
//	│ Billing      │         │ Usage        │
//	│ Worker       │         │ Worker       │
//	│ (batches)    │         │ (batches)    │
//	└──────┬───────┘         └──────┬───────┘
//	       │                         │
//	       │ (retry)                 │ (retry)
//	       ├─────────┐               ├─────────┐
//	       │         │               │         │
//	       ▼         ▼               ▼         ▼
//	 ┌─────────┐ ┌─────┐      ┌──────────┐ ┌─────┐
//	 │  Redis  │ │ DLQ │      │   DB     │ │ DLQ │
//	 │ Billing │ └─────┘      │  Usage   │ └─────┘
//	 └─────────┘              │ Records  │
//	                          └──────────┘
//
// Features:
// - Batch processing (up to 100 items per batch, 5s timeout)
// - Retry with exponential backoff (max 3 retries)
// - Dead-letter queue for failed items
// - Graceful shutdown with queue draining

// Queue defines the interface for message queuing
type Queue interface {
	// Enqueue adds an item to the queue
	Enqueue(ctx context.Context, item interface{}) error

	// Dequeue retrieves items from the queue (up to maxItems)
	// Blocks until at least one item is available or context is cancelled
	Dequeue(ctx context.Context, maxItems int) ([]interface{}, error)

	// DequeueWithTimeout retrieves items with a timeout
	// Returns items if available before timeout, empty slice otherwise
	DequeueWithTimeout(ctx context.Context, maxItems int, timeout time.Duration) ([]interface{}, error)

	// Length returns the current queue length
	Length(ctx context.Context) (int, error)

	// Close shuts down the queue gracefully
	Close() error
}

// DeadLetterQueue defines the interface for handling failed items
type DeadLetterQueue interface {
	// Add adds a failed item to the dead letter queue with error info
	Add(ctx context.Context, item interface{}, err error) error

	// List retrieves items from the dead letter queue
	List(ctx context.Context, maxItems int) ([]DeadLetterItem, error)

	// Remove removes an item from the dead letter queue
	Remove(ctx context.Context, id string) error

	// Close shuts down the dead letter queue
	Close() error
}

// DeadLetterItem represents an item in the dead letter queue
type DeadLetterItem struct {
	ID        string
	Item      interface{}
	Error     string
	Timestamp time.Time
	Retries   int
}

// Config holds queue configuration
type Config struct {
	// BatchSize is the maximum number of items to process in a batch
	BatchSize int

	// BatchTimeout is how long to wait before processing a partial batch
	BatchTimeout time.Duration

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// RetryBackoff is the initial backoff duration for retries
	RetryBackoff time.Duration

	// UseRedis indicates whether to use Redis or in-memory queue
	UseRedis bool

	// RedisAddr is the Redis server address (if UseRedis is true)
	RedisAddr string

	// RedisPassword is the Redis password (if UseRedis is true)
	RedisPassword string

	// RedisDB is the Redis database number (if UseRedis is true)
	RedisDB int

	// QueueName is the name/key for the queue
	QueueName string
}

// DefaultConfig returns default queue configuration
func DefaultConfig(queueName string) *Config {
	return &Config{
		BatchSize:    100,
		BatchTimeout: 5 * time.Second,
		MaxRetries:   3,
		RetryBackoff: 1 * time.Second,
		UseRedis:     false,
		QueueName:    queueName,
	}
}
