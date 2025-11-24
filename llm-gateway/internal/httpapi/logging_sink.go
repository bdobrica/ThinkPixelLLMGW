package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"gateway/internal/logging"
)

// RedisLoggingSink implements logging.Sink using Redis buffer
type RedisLoggingSink struct {
	buffer *logging.RedisBuffer
}

// NewRedisLoggingSink creates a new Redis-backed logging sink
func NewRedisLoggingSink(buffer *logging.RedisBuffer) *RedisLoggingSink {
	return &RedisLoggingSink{
		buffer: buffer,
	}
}

// Enqueue adds a log record to the Redis buffer
func (s *RedisLoggingSink) Enqueue(rec *logging.Record) error {
	// Marshal record to JSON
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal log record: %w", err)
	}

	// Enqueue to Redis buffer (best-effort, use background context)
	ctx := context.Background()
	if err := s.buffer.Enqueue(ctx, data); err != nil {
		// Log error but don't fail the request
		// In production, you'd want to send this to a monitoring system
		fmt.Printf("Warning: failed to enqueue log record: %v\n", err)
		return nil // Don't propagate error to avoid failing requests
	}

	return nil
}
