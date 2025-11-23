# Database Package Usage Guide

This guide demonstrates how to use the database package with its integrated LRU caching layer.

## Quick Start

### 1. Initialize Database Connection

```go
package main

import (
	"context"
	"log"
	"time"
	
	"gateway/internal/storage"
)

func main() {
	// Load configuration (from config package)
	cfg := storage.DefaultDBConfig()
	cfg.Host = "localhost"
	cfg.Database = "llmgateway"
	cfg.User = "postgres"
	cfg.Password = "yourpassword"
	
	// Create database connection with caching
	db, err := storage.NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// Verify connection
	ctx := context.Background()
	if err := db.Health(ctx); err != nil {
		log.Fatalf("Database health check failed: %v", err)
	}
	
	log.Println("Database connected successfully")
}
```

### 2. Using Repositories

#### API Key Repository

```go
// Create repository
apiKeyRepo := db.NewAPIKeyRepository()

// Get API key by hash (cached)
apiKey, err := apiKeyRepo.GetByHash(ctx, "hashed_key_value")
if err != nil {
	log.Printf("Error getting API key: %v", err)
	return
}

// Validate API key
if !apiKey.IsValid() {
	log.Println("API key is invalid or expired")
	return
}

// Check if model is allowed
if !apiKey.AllowsModel("gpt-4") {
	log.Println("Model not allowed for this API key")
	return
}

// Create new API key
newKey := &models.APIKey{
	Name:               "Production Key",
	KeyHash:            "hashed_value",
	AllowedModels:      []string{"gpt-4", "gpt-3.5-turbo"},
	RateLimitPerMinute: 60,
	MonthlyBudgetUSD:   ptrFloat64(1000.0),
	Enabled:            true,
	ExpiresAt:          ptrTime(time.Now().Add(365 * 24 * time.Hour)),
}

if err := apiKeyRepo.Create(ctx, newKey); err != nil {
	log.Printf("Error creating API key: %v", err)
	return
}

// Set metadata (tags, user info, etc.)
err = apiKeyRepo.SetMetadata(ctx, newKey.ID, "tag", "environment", "production")
if err != nil {
	log.Printf("Error setting metadata: %v", err)
}

err = apiKeyRepo.SetMetadata(ctx, newKey.ID, "owner", "email", "user@example.com")
if err != nil {
	log.Printf("Error setting metadata: %v", err)
}
```

#### Model Repository

```go
// Create repository
modelRepo := db.NewModelRepository()

// Get model by name (cached, also checks aliases)
model, err := modelRepo.GetByName(ctx, "gpt-4")
if err != nil {
	log.Printf("Error getting model: %v", err)
	return
}

// Calculate cost for a request
inputTokens := 1000
outputTokens := 500
cost := model.CalculateCost(inputTokens, outputTokens, 0, 0, 0, 0, 0, 0)
log.Printf("Request cost: $%.6f", cost)

// Search for models (fuzzy matching)
models, err := modelRepo.Search(ctx, "gpt", 10)
if err != nil {
	log.Printf("Error searching models: %v", err)
	return
}

for _, m := range models {
	log.Printf("Found model: %s (provider: %s)", m.Name, m.LiteLLMProvider)
}
```

#### Usage Tracking

```go
// Create repository
usageRepo := db.NewUsageRepository()

// Record usage
usage := &models.UsageRecord{
	APIKeyID:         apiKey.ID,
	ModelID:          model.ID,
	ProviderID:       model.ProviderID,
	RequestTimestamp: time.Now(),
	PromptTokens:     1000,
	CompletionTokens: 500,
	TotalTokens:      1500,
	CostUSD:          cost,
	RequestMetadata:  json.RawMessage(`{"user": "test@example.com"}`),
	ResponseMetadata: json.RawMessage(`{"finish_reason": "stop"}`),
}

if err := usageRepo.Create(ctx, usage); err != nil {
	log.Printf("Error recording usage: %v", err)
	return
}

// Get usage statistics
startTime := time.Now().Add(-30 * 24 * time.Hour) // Last 30 days
endTime := time.Now()

totalCost, err := usageRepo.GetTotalCostByAPIKey(ctx, apiKey.ID, startTime, endTime)
if err != nil {
	log.Printf("Error getting total cost: %v", err)
	return
}

log.Printf("Total cost for last 30 days: $%.2f", totalCost)

// Get token usage
promptTokens, completionTokens, totalTokens, err := usageRepo.GetTotalTokensByAPIKey(
	ctx, apiKey.ID, startTime, endTime,
)
if err != nil {
	log.Printf("Error getting token usage: %v", err)
	return
}

log.Printf("Token usage - Prompt: %d, Completion: %d, Total: %d",
	promptTokens, completionTokens, totalTokens)
```

#### Monthly Summaries

```go
// Create repository
summaryRepo := db.NewMonthlyUsageSummaryRepository()

// Refresh summary for current month
now := time.Now()
err = summaryRepo.RefreshSummary(ctx, apiKey.ID, now.Year(), int(now.Month()))
if err != nil {
	log.Printf("Error refreshing summary: %v", err)
	return
}

// Get monthly summary
summary, err := summaryRepo.GetByAPIKeyAndMonth(ctx, apiKey.ID, now.Year(), int(now.Month()))
if err != nil {
	log.Printf("Error getting summary: %v", err)
	return
}

log.Printf("Monthly Summary - Requests: %d, Cost: $%.2f, Tokens: %d",
	summary.TotalRequests, summary.TotalCostUSD, summary.TotalTokens)
```

## Caching Behavior

### Cache-Aside Pattern

All repositories implement the cache-aside pattern:

1. **Read Path** (e.g., `GetByHash`):
   - Check cache first
   - If found, return cached value
   - If not found, query database
   - Cache the result before returning

2. **Write Path** (e.g., `Create`, `Update`, `Delete`):
   - Update database
   - Invalidate cache entry
   - Next read will fetch fresh data from DB

### Cache Configuration

Configure cache via environment variables:

```bash
# API Key cache (default: 1000 keys, 5 minute TTL)
CACHE_API_KEY_SIZE=1000
CACHE_API_KEY_TTL=5m

# Model cache (default: 500 models, 15 minute TTL)
CACHE_MODEL_SIZE=500
CACHE_MODEL_TTL=15m
```

### Cache Eviction

- **LRU Eviction**: When cache is full, oldest accessed entry is removed
- **TTL Expiration**: Entries expire after configured TTL
- **Manual Cleanup**: Periodic cleanup removes expired entries

```go
// Run cleanup every minute (in background goroutine)
go func() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		apiKeyRemoved, modelRemoved := db.CleanupExpiredCacheEntries()
		if apiKeyRemoved > 0 || modelRemoved > 0 {
			log.Printf("Cleaned up %d API keys and %d models from cache",
				apiKeyRemoved, modelRemoved)
		}
	}
}()
```

## Monitoring

### Database Statistics

```go
stats := db.GetStats()

log.Printf("Database Stats:")
log.Printf("  Open Connections: %d/%d", stats.OpenConnections, stats.MaxOpenConnections)
log.Printf("  In Use: %d, Idle: %d", stats.InUse, stats.Idle)
log.Printf("  Wait Count: %d, Wait Duration: %v", stats.WaitCount, stats.WaitDuration)

log.Printf("API Key Cache Stats:")
log.Printf("  Size: %d, Capacity: %d", stats.APIKeyCacheStats.Size, stats.APIKeyCacheStats.Capacity)
log.Printf("  Hits: %d, Misses: %d", stats.APIKeyCacheStats.Hits, stats.APIKeyCacheStats.Misses)
log.Printf("  Hit Rate: %.2f%%", stats.APIKeyCacheStats.HitRate*100)
log.Printf("  Evictions: %d, Expirations: %d", 
	stats.APIKeyCacheStats.Evictions, stats.APIKeyCacheStats.Expirations)

log.Printf("Model Cache Stats:")
log.Printf("  Size: %d, Capacity: %d", stats.ModelCacheStats.Size, stats.ModelCacheStats.Capacity)
log.Printf("  Hit Rate: %.2f%%", stats.ModelCacheStats.HitRate*100)
```

## Best Practices

### 1. Connection Pooling

```go
cfg := storage.DefaultDBConfig()
cfg.MaxOpenConns = 25    // Limit concurrent connections
cfg.MaxIdleConns = 5     // Keep idle connections for reuse
cfg.ConnMaxLifetime = 5 * time.Minute  // Recycle connections
cfg.ConnMaxIdleTime = 1 * time.Minute  // Close idle connections
```

### 2. Context Timeouts

```go
// Set timeout for operations
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

apiKey, err := apiKeyRepo.GetByHash(ctx, keyHash)
```

### 3. Transaction Usage

```go
// Start transaction
tx, err := db.BeginTx(ctx, nil)
if err != nil {
	return err
}
defer tx.Rollback()

// Perform multiple operations
// ... (use tx.Exec, tx.Query, etc.)

// Commit transaction
if err := tx.Commit(); err != nil {
	return err
}
```

### 4. Cache Invalidation

```go
// After external updates, manually invalidate cache
apiKeyRepo.InvalidateCache(keyHash)
modelRepo.InvalidateCache(modelName)
```

### 5. Pagination

```go
// List API keys with pagination
limit := 50
offset := 0

apiKeys, err := apiKeyRepo.List(ctx, limit, offset)
if err != nil {
	return err
}

// Next page
offset += limit
apiKeys, err = apiKeyRepo.List(ctx, limit, offset)
```

## Performance Characteristics

### Latency Goals

- **Cache Hit**: < 1ms (in-memory lookup)
- **Cache Miss**: < 50ms (single DB query)
- **Total Request**: < 200ms (including all operations)

### Throughput

With default settings:
- **API Key lookups**: ~50,000 req/sec (cached)
- **Model lookups**: ~100,000 req/sec (cached)
- **Database writes**: Limited by DB performance (~1,000 req/sec)

### Memory Usage

- **API Key cache**: ~1000 keys × ~500 bytes = ~500 KB
- **Model cache**: ~500 models × ~2 KB = ~1 MB
- **Total**: ~1.5 MB for caches

## Error Handling

```go
import "errors"

apiKey, err := apiKeyRepo.GetByHash(ctx, keyHash)
if err != nil {
	switch {
	case errors.Is(err, storage.ErrAPIKeyNotFound):
		// Handle not found
		log.Println("API key not found")
	case errors.Is(err, context.DeadlineExceeded):
		// Handle timeout
		log.Println("Database query timed out")
	default:
		// Handle other errors
		log.Printf("Unexpected error: %v", err)
	}
	return
}
```

## Utilities

```go
// Helper functions for pointer types
func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrInt(v int) *int {
	return &v
}
```
