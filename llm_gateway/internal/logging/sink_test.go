package logging

import (
	"context"
	"testing"
	"time"
)

func TestNoopSink(t *testing.T) {
	sink := NewNoopSink()

	rec := &LogRecord{
		Timestamp: time.Now(),
		RequestID: "test-123",
		APIKeyID:  "key-456",
		Provider:  "openai",
		Model:     "gpt-4",
		CostUSD:   0.05,
	}

	err := sink.Enqueue(rec)
	if err != nil {
		t.Errorf("Expected no error from NoopSink.Enqueue, got %v", err)
	}

	err = sink.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected no error from NoopSink.Shutdown, got %v", err)
	}
}

func TestS3SinkConfig(t *testing.T) {
	config := S3SinkConfig{
		BufferSize:    1000,
		FlushSize:     100,
		FlushInterval: 5 * time.Minute,
		S3Bucket:      "test-bucket",
		S3Region:      "us-east-1",
		S3Prefix:      "logs/",
		PodName:       "test-pod",
	}

	if config.BufferSize != 1000 {
		t.Errorf("Expected buffer size 1000, got %d", config.BufferSize)
	}

	if config.FlushSize != 100 {
		t.Errorf("Expected flush size 100, got %d", config.FlushSize)
	}

	if config.S3Bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got '%s'", config.S3Bucket)
	}
}

// Note: Full integration tests for S3Sink require AWS credentials and actual S3 bucket
// These should be run separately with appropriate environment setup
