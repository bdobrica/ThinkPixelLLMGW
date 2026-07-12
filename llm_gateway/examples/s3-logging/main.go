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

// This example buffers request logs in Redis and flushes them to S3.
func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	buffer := logging.NewRedisBuffer(redisClient, logging.RedisBufferConfig{
		QueueKey:  "demo:logs",
		MaxSize:   10000,
		BatchSize: 100,
	})

	var sink logging.Sink
	if cfg.LoggingSink.Enabled && cfg.LoggingSink.S3Bucket != "" {
		sink, err = logging.NewS3Sink(ctx, logging.S3SinkConfig{
			Enabled:       cfg.LoggingSink.Enabled,
			BufferSize:    cfg.LoggingSink.BufferSize,
			FlushSize:     cfg.LoggingSink.FlushSize,
			FlushInterval: cfg.LoggingSink.FlushInterval,
			S3Bucket:      cfg.LoggingSink.S3Bucket,
			S3Region:      cfg.LoggingSink.S3Region,
			S3Prefix:      cfg.LoggingSink.S3Prefix,
			PodName:       cfg.LoggingSink.PodName,
		}, buffer)
		if err != nil {
			log.Fatalf("Failed to create S3 sink: %v", err)
		}
		fmt.Printf("S3 logging sink initialized for s3://%s/%s\n", cfg.LoggingSink.S3Bucket, cfg.LoggingSink.S3Prefix)
	} else {
		sink = logging.NewNoopSink()
		fmt.Println("Using no-op sink because S3 logging is disabled")
	}

	fmt.Println("Enqueuing sample log records...")
	for i := 0; i < 10; i++ {
		record := &logging.LogRecord{
			Timestamp:  time.Now(),
			RequestID:  fmt.Sprintf("req-%d", i),
			APIKeyID:   "demo-api-key",
			APIKeyName: "Demo Key",
			Provider:   "openai",
			Model:      "gpt-4",
			Alias:      "gpt4",
			Tags:       map[string]string{"env": "demo", "user": "example"},
			ProviderMs: 1234,
			GatewayMs:  1250,
			CostUSD:    0.05,
			RequestPayload: map[string]interface{}{
				"messages": []map[string]string{{"role": "user", "content": "Hello!"}},
			},
			ResponsePayload: map[string]interface{}{
				"choices": []map[string]interface{}{{
					"message": map[string]string{"role": "assistant", "content": "Hello! How can I help you?"},
				}},
			},
		}

		if err := sink.Enqueue(record); err != nil {
			log.Printf("Failed to enqueue record: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := sink.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
