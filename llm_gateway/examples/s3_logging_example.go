package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"llm_gateway/internal/config"
	"llm_gateway/internal/logging"
)

// This example demonstrates how to use the S3 logging sink
// to buffer request logs in Redis and flush them to S3 periodically.
//
// Prerequisites:
// - Redis running (for buffering logs)
// - AWS credentials configured (via ~/.aws/credentials, IAM role, or env vars)
// - S3 bucket created
// - Environment variables set (see below)

func main() {
	ctx := context.Background()

	// Load configuration (or set manually for testing)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Redis client for buffering
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create Redis buffer
	buffer := logging.NewRedisBuffer(redisClient, logging.RedisBufferConfig{
		QueueKey:  "demo:logs",
		MaxSize:   10000,
		BatchSize: 100,
	})

	// Create S3 sink if enabled
	var sink logging.Sink
	if cfg.LoggingSink.Enabled && cfg.LoggingSink.S3Bucket != "" {
		sinkConfig := logging.S3SinkConfig{
			Enabled:       cfg.LoggingSink.Enabled,
			BufferSize:    cfg.LoggingSink.BufferSize,
			FlushSize:     cfg.LoggingSink.FlushSize,
			FlushInterval: cfg.LoggingSink.FlushInterval,
			S3Bucket:      cfg.LoggingSink.S3Bucket,
			S3Region:      cfg.LoggingSink.S3Region,
			S3Prefix:      cfg.LoggingSink.S3Prefix,
			PodName:       cfg.LoggingSink.PodName,
		}

		sink, err = logging.NewS3Sink(ctx, sinkConfig, buffer)
		if err != nil {
			log.Fatalf("Failed to create S3 sink: %v", err)
		}
		defer sink.Shutdown(ctx)

		fmt.Println("S3 logging sink initialized successfully")
		fmt.Printf("  Bucket: %s\n", cfg.LoggingSink.S3Bucket)
		fmt.Printf("  Region: %s\n", cfg.LoggingSink.S3Region)
		fmt.Printf("  Prefix: %s\n", cfg.LoggingSink.S3Prefix)
		fmt.Printf("  Flush size: %d records\n", cfg.LoggingSink.FlushSize)
		fmt.Printf("  Flush interval: %s\n", cfg.LoggingSink.FlushInterval)
	} else {
		// Use no-op sink if S3 is not enabled
		sink = logging.NewNoopSink()
		fmt.Println("Using no-op sink (S3 logging disabled)")
	}

	// Simulate some log records
	fmt.Println("\nEnqueuing sample log records...")
	for i := 0; i < 10; i++ {
		record := &logging.LogRecord{
			Timestamp:  time.Now(),
			RequestID:  fmt.Sprintf("req-%d", i),
			APIKeyID:   "demo-api-key",
			APIKeyName: "Demo Key",
			Provider:   "openai",
			Model:      "gpt-4",
			Alias:      "gpt4",
			Tags: map[string]string{
				"env":  "demo",
				"user": "example",
			},
			ProviderMs: 1234,
			GatewayMs:  1250,
			CostUSD:    0.05,
			RequestPayload: map[string]interface{}{
				"messages": []map[string]string{
					{"role": "user", "content": "Hello!"},
				},
			},
			ResponsePayload: map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]string{
							"role":    "assistant",
							"content": "Hello! How can I help you?",
						},
					},
				},
			},
		}

		if err := sink.Enqueue(record); err != nil {
			log.Printf("Failed to enqueue record: %v", err)
		} else {
			fmt.Printf("  Enqueued: %s\n", record.RequestID)
		}

		// Small delay to simulate real requests
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\nRecords enqueued. Waiting for flush...")
	fmt.Println("(In production, records would be flushed based on size or time interval)")

	// Wait a bit to see periodic flush
	time.Sleep(2 * time.Second)

	// Graceful shutdown will flush remaining records
	fmt.Println("\nShutting down and flushing remaining records...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := sink.Shutdown(shutdownCtx); err != nil {
		log.Printf("Warning: shutdown error: %v", err)
	}

	fmt.Println("Done!")
}
