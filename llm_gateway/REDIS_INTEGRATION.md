# Redis Integration Guide

This guide demonstrates how to use Redis for rate limiting, billing cache, and log buffering in the LLM Gateway.

## Overview

Redis is used for three main purposes:
1. **Rate Limiting**: Distributed rate limiting using sliding window algorithm
2. **Billing Cache**: Track running costs with atomic increments and periodic DB sync
3. **Log Buffer**: Queue log records for batch upload to S3

## Quick Start

### 1. Initialize Redis Client

```go
package main

import (
	"context"
	"log"
	
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/storage"
)

func main() {
	// Create Redis client
	cfg := storage.DefaultRedisConfig()
	cfg.Address = "localhost:6379"
	cfg.Password = "" // Set if using auth
	
	redis, err := storage.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()
	
	// Verify connection
	ctx := context.Background()
	if err := redis.Health(ctx); err != nil {
		log.Fatalf("Redis health check failed: %v", err)
	}
	
	log.Println("Redis connected successfully")
}
```

### 2. Rate Limiting

#### Sliding Window Rate Limiter

```go
import "github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/ratelimit"

// Create rate limiter
limiter := ratelimit.NewRateLimiter(redis.Client())

// Check if request is allowed
apiKeyID := "550e8400-e29b-41d4-a716-446655440000"
limit := 60 // 60 requests per minute

allowed, err := limiter.Allow(ctx, apiKeyID, limit)
if err != nil {
	log.Printf("Rate limit check failed: %v", err)
	return
}

if !allowed {
	log.Println("Rate limit exceeded")
	// Return 429 Too Many Requests
	return
}

// Get current usage
usage, err := limiter.GetCurrentUsage(ctx, apiKeyID)
if err != nil {
	log.Printf("Failed to get usage: %v", err)
}
log.Printf("Current usage: %d/%d requests", usage, limit)

// Reset rate limit (admin operation)
if err := limiter.Reset(ctx, apiKeyID); err != nil {
	log.Printf("Failed to reset: %v", err)
}
```

#### Token Bucket Rate Limiter

```go
// Create token bucket limiter
tbLimiter := ratelimit.NewTokenBucketLimiter(redis.Client())

// Allow with burst capability
rate := 60   // 60 tokens per minute
burst := 100 // Allow burst up to 100 tokens

allowed, err := tbLimiter.Allow(ctx, apiKeyID, rate, burst)
if err != nil {
	log.Printf("Token bucket check failed: %v", err)
	return
}

if !allowed {
	log.Println("No tokens available")
	return
}

// Check remaining tokens
remaining, err := tbLimiter.GetRemainingTokens(ctx, apiKeyID, rate, burst)
if err != nil {
	log.Printf("Failed to get remaining tokens: %v", err)
}
log.Printf("Remaining tokens: %.2f", remaining)
```

### 3. Billing Cache

```go
import (
	"time"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/billing"
)

// Create billing service
syncFreq := 5 * time.Minute // Sync to DB every 5 minutes
billingService := billing.NewRedisBillingService(
	redis.Client(),
	db,
	syncFreq,
)

// Check if within budget
apiKeyID := "550e8400-e29b-41d4-a716-446655440000"
if !billingService.WithinBudget(ctx, apiKeyID) {
	log.Println("Monthly budget exceeded")
	// Return 402 Payment Required
	return
}

// Add usage after request completion
cost := 0.002 // $0.002
if err := billingService.AddUsage(ctx, apiKeyID, cost); err != nil {
	log.Printf("Failed to add usage: %v", err)
}

// Get current month's spending
spending, err := billingService.GetMonthlySpending(ctx, apiKeyID)
if err != nil {
	log.Printf("Failed to get spending: %v", err)
	return
}
log.Printf("Current month spending: $%.4f", spending)

// Get specific month's spending
spending, err = billingService.GetSpending(ctx, apiKeyID, 2025, 1)
if err != nil {
	log.Printf("Failed to get spending: %v", err)
	return
}
log.Printf("January 2025 spending: $%.4f", spending)

// Graceful shutdown (syncs to DB)
if err := billingService.Shutdown(ctx); err != nil {
	log.Printf("Failed to shutdown billing service: %v", err)
}
```

### 4. Log Buffer

```go
import "github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/logging"

// Create Redis buffer
bufferCfg := logging.DefaultRedisBufferConfig()
bufferCfg.MaxSize = 100000  // 100k max entries
bufferCfg.BatchSize = 100   // Dequeue 100 at a time

buffer := logging.NewRedisBuffer(redis.Client(), bufferCfg)

// Enqueue single log record
record := &logging.LogRecord{
	APIKeyID:     "550e8400-e29b-41d4-a716-446655440000",
	Model:        "gpt-4",
	PromptTokens: 100,
	ResponseTokens: 50,
	CostUSD:      0.002,
	// ... other fields
}

if err := buffer.Enqueue(ctx, record); err != nil {
	log.Printf("Failed to enqueue: %v", err)
}

// Enqueue batch
records := []*logging.LogRecord{record1, record2, record3}
if err := buffer.EnqueueBatch(ctx, records); err != nil {
	log.Printf("Failed to enqueue batch: %v", err)
}

// Dequeue batch for processing
records, err := buffer.Dequeue(ctx, 100)
if err != nil {
	log.Printf("Failed to dequeue: %v", err)
	return
}
log.Printf("Dequeued %d records", len(records))

// Peek without removing
records, err = buffer.Peek(ctx, 10)
if err != nil {
	log.Printf("Failed to peek: %v", err)
	return
}
log.Printf("Next %d records in queue", len(records))

// Wait for records (blocking)
timeout := 30 * time.Second
records, err = buffer.WaitForRecords(ctx, timeout)
if err != nil {
	log.Printf("Failed to wait: %v", err)
	return
}
if records == nil {
	log.Println("Timeout - no records available")
	return
}

// Get buffer statistics
stats, err := buffer.GetStats(ctx)
if err != nil {
	log.Printf("Failed to get stats: %v", err)
	return
}
log.Printf("Buffer stats - Size: %d/%d, Batch: %d",
	stats.QueueSize, stats.MaxSize, stats.BatchSize)
```

## Configuration

### Environment Variables

```bash
# Redis connection
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=your_password_here
REDIS_DB=0

# Redis pool settings
REDIS_POOL_SIZE=10
REDIS_MIN_IDLE_CONNS=2

# Redis timeouts
REDIS_DIAL_TIMEOUT=5s
REDIS_READ_TIMEOUT=3s
REDIS_WRITE_TIMEOUT=3s
```

### Configuration in Code

```go
import "github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/config"

cfg, err := config.Load()
if err != nil {
	log.Fatalf("Failed to load config: %v", err)
}

// Create Redis client from config
redisConfig := storage.RedisConfig{
	Address:      cfg.Redis.Address,
	Password:     cfg.Redis.Password,
	DB:           cfg.Redis.DB,
	PoolSize:     cfg.Redis.PoolSize,
	MinIdleConns: cfg.Redis.MinIdleConns,
	DialTimeout:  cfg.Redis.DialTimeout,
	ReadTimeout:  cfg.Redis.ReadTimeout,
	WriteTimeout: cfg.Redis.WriteTimeout,
}

redis, err := storage.NewRedisClient(redisConfig)
```

## Redis Data Structures

### Rate Limiting Keys

Sliding window uses Redis sorted sets:

```
Key: ratelimit:<api_key_id>
Type: Sorted Set
Score: Timestamp (milliseconds)
Member: <timestamp>:<sequence>
TTL: 2 minutes
```

Token bucket uses simple strings:

```
Key: tokenbucket:<api_key_id>:tokens
Type: String
Value: Float (remaining tokens)
TTL: 2 minutes

Key: tokenbucket:<api_key_id>:last
Type: String
Value: Timestamp (milliseconds)
TTL: 2 minutes
```

### Billing Keys

```
Key: cost:<api_key_id>:<year>:<month>
Type: String
Value: Float (total cost in USD)
TTL: 60 days

Example: cost:550e8400-e29b-41d4-a716-446655440000:2025:01
```

### Log Buffer Keys

```
Key: logs:queue
Type: List
Value: JSON-serialized LogRecord
Max Size: 100,000 entries (configurable)
```

## Performance Characteristics

### Rate Limiting

- **Latency**: < 5ms for Allow check
- **Throughput**: ~10,000 checks/sec per Redis instance
- **Accuracy**: Exact sliding window (no approximation)
- **Memory**: ~100 bytes per API key

### Billing Cache

- **Latency**: < 2ms for WithinBudget check
- **Throughput**: ~20,000 operations/sec
- **Sync Frequency**: Configurable (default 5 minutes)
- **Memory**: ~50 bytes per API key per month

### Log Buffer

- **Latency**: < 3ms for enqueue
- **Throughput**: ~15,000 enqueues/sec
- **Batch Dequeue**: < 10ms for 100 records
- **Memory**: ~1 KB per log record

## Redis Cluster Support

For high availability and horizontal scaling:

```go
// Create cluster client
clusterCfg := storage.ClusterConfig{
	Addrs: []string{
		"redis-node-1:6379",
		"redis-node-2:6379",
		"redis-node-3:6379",
	},
	Password:     "cluster_password",
	PoolSize:     20,
	MinIdleConns: 5,
	DialTimeout:  5 * time.Second,
	ReadTimeout:  3 * time.Second,
	WriteTimeout: 3 * time.Second,
}

cluster, err := storage.NewClusterClient(clusterCfg)
if err != nil {
	log.Fatalf("Failed to connect to Redis cluster: %v", err)
}
defer cluster.Close()

// Use cluster client
limiter := ratelimit.NewRateLimiter(cluster.Client())
```

## Best Practices

### 1. Connection Pooling

```go
cfg := storage.DefaultRedisConfig()
cfg.PoolSize = 20        // Concurrent connections
cfg.MinIdleConns = 5     // Keep connections warm
```

### 2. Error Handling

```go
allowed, err := limiter.Allow(ctx, apiKeyID, limit)
if err != nil {
	// Log error
	log.Printf("Rate limit check failed: %v", err)
	
	// Fail open (allow request) or fail closed (deny)?
	// Recommendation: Fail open to prevent service disruption
	allowed = true
}
```

### 3. Context Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

allowed, err := limiter.Allow(ctx, apiKeyID, limit)
```

### 4. Monitoring

```go
// Get Redis pool statistics
stats := redis.GetStats()
log.Printf("Redis Stats - Hits: %d, Misses: %d, Timeouts: %d",
	stats.Hits, stats.Misses, stats.Timeouts)
log.Printf("Connections - Total: %d, Idle: %d, Stale: %d",
	stats.TotalConns, stats.IdleConns, stats.StaleConns)
```

### 5. Graceful Shutdown

```go
// Flush pending operations
if err := billingService.Shutdown(ctx); err != nil {
	log.Printf("Billing shutdown error: %v", err)
}

// Close Redis connection
if err := redis.Close(); err != nil {
	log.Printf("Redis close error: %v", err)
}
```

## Docker Example

```yaml
version: '3.8'
services:
  gateway:
    image: llmgateway:latest
    environment:
      - REDIS_ADDRESS=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
    depends_on:
      - redis
  
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    ports:
      - "6379:6379"

volumes:
  redis_data:
```

## Kubernetes Example

```yaml
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  ports:
    - port: 6379
  selector:
    app: redis

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command: ["redis-server", "--requirepass", "$(REDIS_PASSWORD)"]
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: redis-secret
              key: password
        ports:
        - containerPort: 6379
        volumeMounts:
        - name: redis-storage
          mountPath: /data
      volumes:
      - name: redis-storage
        persistentVolumeClaim:
          claimName: redis-pvc
```

## Troubleshooting

### Issue: Connection timeouts
**Solution**: Increase `REDIS_DIAL_TIMEOUT` or check network connectivity

### Issue: Rate limit not resetting
**Solution**: Check that TTL is set correctly (2 minutes for rate limit keys)

### Issue: Billing sync not happening
**Solution**: Verify billing service worker is running (check logs for "Failed to sync")

### Issue: Log buffer overflow
**Solution**: Increase `MaxSize` or improve S3 upload frequency

### Issue: High memory usage
**Solution**: 
- Reduce rate limit window size
- Decrease log buffer max size
- Enable Redis maxmemory policy (`volatile-lru`)
