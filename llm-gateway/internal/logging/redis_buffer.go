package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisBuffer implements a Redis-backed buffer for log records
type RedisBuffer struct {
	client    *redis.Client
	queueKey  string
	maxSize   int64 // Maximum queue size (0 = unlimited)
	batchSize int   // Number of records to retrieve at once
}

// RedisBufferConfig holds configuration for Redis buffer
type RedisBufferConfig struct {
	QueueKey  string // Redis list key for the queue
	MaxSize   int64  // Maximum queue size (older entries dropped when full)
	BatchSize int    // Number of records to dequeue at once
}

// DefaultRedisBufferConfig returns default configuration
func DefaultRedisBufferConfig() RedisBufferConfig {
	return RedisBufferConfig{
		QueueKey:  "logs:queue",
		MaxSize:   100000, // 100k log entries max
		BatchSize: 100,    // Dequeue 100 at a time
	}
}

// NewRedisBuffer creates a new Redis-backed log buffer
func NewRedisBuffer(client *redis.Client, cfg RedisBufferConfig) *RedisBuffer {
	return &RedisBuffer{
		client:    client,
		queueKey:  cfg.QueueKey,
		maxSize:   cfg.MaxSize,
		batchSize: cfg.BatchSize,
	}
}

// Enqueue adds a log record to the Redis queue
func (rb *RedisBuffer) Enqueue(ctx context.Context, record *LogRecord) error {
	// Serialize record to JSON
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal log record: %w", err)
	}

	// Use Lua script for atomic enqueue with size limit
	if rb.maxSize > 0 {
		script := redis.NewScript(`
			local key = KEYS[1]
			local value = ARGV[1]
			local max_size = tonumber(ARGV[2])
			
			-- Add to end of list
			redis.call('RPUSH', key, value)
			
			-- Trim to max size (remove oldest entries from left)
			local len = redis.call('LLEN', key)
			if len > max_size then
				redis.call('LTRIM', key, len - max_size, -1)
			end
			
			return len
		`)

		_, err = script.Run(ctx, rb.client, []string{rb.queueKey}, data, rb.maxSize).Result()
		if err != nil {
			return fmt.Errorf("failed to enqueue log record: %w", err)
		}
	} else {
		// No size limit, just push
		if err := rb.client.RPush(ctx, rb.queueKey, data).Err(); err != nil {
			return fmt.Errorf("failed to enqueue log record: %w", err)
		}
	}

	return nil
}

// EnqueueBatch adds multiple log records atomically
func (rb *RedisBuffer) EnqueueBatch(ctx context.Context, records []*LogRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Serialize all records
	values := make([]interface{}, len(records))
	for i, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("failed to marshal log record %d: %w", i, err)
		}
		values[i] = data
	}

	// Push all at once
	pipe := rb.client.Pipeline()
	pipe.RPush(ctx, rb.queueKey, values...)

	// Trim if needed
	if rb.maxSize > 0 {
		pipe.LTrim(ctx, rb.queueKey, -rb.maxSize, -1)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue batch: %w", err)
	}

	return nil
}

// Dequeue removes and returns a batch of log records
func (rb *RedisBuffer) Dequeue(ctx context.Context, count int) ([]*LogRecord, error) {
	if count <= 0 {
		count = rb.batchSize
	}

	// Use Lua script for atomic dequeue
	script := redis.NewScript(`
		local key = KEYS[1]
		local count = tonumber(ARGV[1])
		
		-- Get records from left (oldest first)
		local records = redis.call('LRANGE', key, 0, count - 1)
		
		-- Remove those records
		if #records > 0 then
			redis.call('LTRIM', key, #records, -1)
		end
		
		return records
	`)

	result, err := script.Run(ctx, rb.client, []string{rb.queueKey}, count).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue: %w", err)
	}

	// Deserialize records
	records := make([]*LogRecord, 0, len(result))
	for i, data := range result {
		var record LogRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record %d: %w", i, err)
		}
		records = append(records, &record)
	}

	return records, nil
}

// Peek returns records without removing them
func (rb *RedisBuffer) Peek(ctx context.Context, count int) ([]*LogRecord, error) {
	if count <= 0 {
		count = rb.batchSize
	}

	// Get records without removing
	result, err := rb.client.LRange(ctx, rb.queueKey, 0, int64(count-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to peek: %w", err)
	}

	// Deserialize records
	records := make([]*LogRecord, 0, len(result))
	for i, data := range result {
		var record LogRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal record %d: %w", i, err)
		}
		records = append(records, &record)
	}

	return records, nil
}

// Size returns the current queue size
func (rb *RedisBuffer) Size(ctx context.Context) (int64, error) {
	return rb.client.LLen(ctx, rb.queueKey).Result()
}

// Clear removes all records from the queue
func (rb *RedisBuffer) Clear(ctx context.Context) error {
	return rb.client.Del(ctx, rb.queueKey).Err()
}

// IsEmpty checks if the queue is empty
func (rb *RedisBuffer) IsEmpty(ctx context.Context) (bool, error) {
	size, err := rb.Size(ctx)
	if err != nil {
		return false, err
	}
	return size == 0, nil
}

// WaitForRecords blocks until at least one record is available or timeout
func (rb *RedisBuffer) WaitForRecords(ctx context.Context, timeout time.Duration) ([]*LogRecord, error) {
	// Use BLPOP for blocking wait
	result, err := rb.client.BLPop(ctx, timeout, rb.queueKey).Result()
	if err == redis.Nil {
		// Timeout - no records available
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to wait for records: %w", err)
	}

	// result[0] is the key name, result[1] is the value
	if len(result) < 2 {
		return nil, nil
	}

	var record LogRecord
	if err := json.Unmarshal([]byte(result[1]), &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return []*LogRecord{&record}, nil
}

// Stats returns buffer statistics
type BufferStats struct {
	QueueSize int64
	MaxSize   int64
	BatchSize int
}

// GetStats returns current buffer statistics
func (rb *RedisBuffer) GetStats(ctx context.Context) (BufferStats, error) {
	size, err := rb.Size(ctx)
	if err != nil {
		return BufferStats{}, err
	}

	return BufferStats{
		QueueSize: size,
		MaxSize:   rb.maxSize,
		BatchSize: rb.batchSize,
	}, nil
}
