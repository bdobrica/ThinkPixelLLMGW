package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisQueue implements Queue using Redis lists
type RedisQueue struct {
	client *redis.Client
	config *Config
	qKey   string
}

// NewRedisQueue creates a new Redis-backed queue
func NewRedisQueue(config *Config) (*RedisQueue, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisQueue{
		client: client,
		config: config,
		qKey:   fmt.Sprintf("queue:%s", config.QueueName),
	}, nil
}

// Enqueue adds an item to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, item interface{}) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	if err := q.client.RPush(ctx, q.qKey, data).Err(); err != nil {
		return fmt.Errorf("failed to push to Redis: %w", err)
	}

	return nil
}

// Dequeue retrieves items from the queue
func (q *RedisQueue) Dequeue(ctx context.Context, maxItems int) ([]interface{}, error) {
	// Block until at least one item is available
	result, err := q.client.BLPop(ctx, 0, q.qKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to pop from Redis: %w", err)
	}

	// result[0] is the key, result[1] is the value
	items := []interface{}{json.RawMessage(result[1])}

	// Try to get more items without blocking
	for len(items) < maxItems {
		result, err := q.client.LPop(ctx, q.qKey).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return items, nil // Return what we have so far
		}
		items = append(items, json.RawMessage(result))
	}

	return items, nil
}

// DequeueWithTimeout retrieves items with a timeout
func (q *RedisQueue) DequeueWithTimeout(ctx context.Context, maxItems int, timeout time.Duration) ([]interface{}, error) {
	// Block until item is available or timeout
	result, err := q.client.BLPop(ctx, timeout, q.qKey).Result()
	if err == redis.Nil {
		return []interface{}{}, nil // Timeout, no items
	}
	if err != nil {
		return nil, fmt.Errorf("failed to pop from Redis: %w", err)
	}

	// result[0] is the key, result[1] is the value
	items := []interface{}{json.RawMessage(result[1])}

	// Try to get more items without blocking
	for len(items) < maxItems {
		result, err := q.client.LPop(ctx, q.qKey).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return items, nil // Return what we have so far
		}
		items = append(items, json.RawMessage(result))
	}

	return items, nil
}

// Length returns the current queue length
func (q *RedisQueue) Length(ctx context.Context) (int, error) {
	length, err := q.client.LLen(ctx, q.qKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue length: %w", err)
	}
	return int(length), nil
}

// Close shuts down the queue
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// RedisDeadLetterQueue implements DeadLetterQueue using Redis hashes
type RedisDeadLetterQueue struct {
	client *redis.Client
	dlKey  string
}

// NewRedisDeadLetterQueue creates a new Redis-backed dead letter queue
func NewRedisDeadLetterQueue(config *Config) (*RedisDeadLetterQueue, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisDeadLetterQueue{
		client: client,
		dlKey:  fmt.Sprintf("dlq:%s", config.QueueName),
	}, nil
}

// Add adds a failed item to the dead letter queue
func (q *RedisDeadLetterQueue) Add(ctx context.Context, item interface{}, err error) error {
	dlItem := DeadLetterItem{
		ID:        generateID(),
		Item:      item,
		Error:     err.Error(),
		Timestamp: time.Now(),
		Retries:   0,
	}

	data, marshalErr := json.Marshal(dlItem)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal dead letter item: %w", marshalErr)
	}

	if err := q.client.HSet(ctx, q.dlKey, dlItem.ID, data).Err(); err != nil {
		return fmt.Errorf("failed to add to dead letter queue: %w", err)
	}

	return nil
}

// List retrieves items from the dead letter queue
func (q *RedisDeadLetterQueue) List(ctx context.Context, maxItems int) ([]DeadLetterItem, error) {
	// Get all items from the hash
	results, err := q.client.HGetAll(ctx, q.dlKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list dead letter items: %w", err)
	}

	items := make([]DeadLetterItem, 0, len(results))
	for _, data := range results {
		var dlItem DeadLetterItem
		if err := json.Unmarshal([]byte(data), &dlItem); err != nil {
			continue // Skip malformed items
		}
		items = append(items, dlItem)

		if maxItems > 0 && len(items) >= maxItems {
			break
		}
	}

	return items, nil
}

// Remove removes an item from the dead letter queue
func (q *RedisDeadLetterQueue) Remove(ctx context.Context, id string) error {
	if err := q.client.HDel(ctx, q.dlKey, id).Err(); err != nil {
		return fmt.Errorf("failed to remove from dead letter queue: %w", err)
	}
	return nil
}

// Close shuts down the dead letter queue
func (q *RedisDeadLetterQueue) Close() error {
	return q.client.Close()
}
