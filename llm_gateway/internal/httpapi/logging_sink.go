package httpapi

import (
	"context"
	"fmt"

	"llm_gateway/internal/logging"
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
func (s *RedisLoggingSink) Enqueue(rec *logging.LogRecord) error {
	ctx := context.Background()

	// Enqueue to Redis buffer
	if err := s.buffer.Enqueue(ctx, rec); err != nil {
		return fmt.Errorf("failed to enqueue log record: %w", err)
	}

	return nil
}
