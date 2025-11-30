package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// getRedisAddr returns the Redis address from environment or default
func getRedisAddr() string {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return addr
}

// isRedisAvailable checks if Redis is available for testing
func isRedisAvailable(t *testing.T) bool {
	client := redis.NewClient(&redis.Options{
		Addr: getRedisAddr(),
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return false
	}
	return true
}

// cleanupRedisQueue removes all items from a test queue
func cleanupRedisQueue(t *testing.T, config *Config) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})
	defer client.Close()

	ctx := context.Background()
	qKey := "queue:" + config.QueueName
	dlKey := "dlq:" + config.QueueName

	client.Del(ctx, qKey)
	client.Del(ctx, dlKey)
}

func TestRedisQueue_EnqueueDequeue(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-basic")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15 // Use separate DB for tests

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	q, err := NewRedisQueue(config)
	if err != nil {
		t.Fatalf("NewRedisQueue failed: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Test single item
	item := map[string]string{"key": "value"}
	err = q.Enqueue(ctx, item)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	items, err := q.Dequeue(ctx, 1)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
}

func TestRedisQueue_MultipleBatch(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-batch")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15
	config.BatchSize = 5

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	q, err := NewRedisQueue(config)
	if err != nil {
		t.Fatalf("NewRedisQueue failed: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Enqueue multiple items
	for i := 0; i < 10; i++ {
		err := q.Enqueue(ctx, map[string]int{"value": i})
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Verify length
	length, err := q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 10 {
		t.Errorf("Expected length 10, got %d", length)
	}

	// Dequeue in batches
	items, err := q.Dequeue(ctx, 5)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items, got %d", len(items))
	}

	// Verify remaining length
	length, err = q.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}
	if length != 5 {
		t.Errorf("Expected length 5 after first dequeue, got %d", length)
	}
}

func TestRedisQueue_DequeueWithTimeout(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-timeout")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	q, err := NewRedisQueue(config)
	if err != nil {
		t.Fatalf("NewRedisQueue failed: %v", err)
	}
	defer q.Close()

	ctx := context.Background()

	// Test timeout with no items
	start := time.Now()
	items, err := q.DequeueWithTimeout(ctx, 1, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("DequeueWithTimeout failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items on timeout, got %d", len(items))
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected timeout, but returned early: %v", elapsed)
	}

	// Test with items available
	err = q.Enqueue(ctx, "test")
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	items, err = q.DequeueWithTimeout(ctx, 1, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("DequeueWithTimeout failed: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
}

func TestRedisQueue_Persistence(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-persist")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	ctx := context.Background()

	// Create queue, add items, close
	q1, err := NewRedisQueue(config)
	if err != nil {
		t.Fatalf("NewRedisQueue failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		err := q1.Enqueue(ctx, i)
		if err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	err = q1.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Create new queue instance - items should still be there
	q2, err := NewRedisQueue(config)
	if err != nil {
		t.Fatalf("NewRedisQueue failed: %v", err)
	}
	defer q2.Close()

	length, err := q2.Length(ctx)
	if err != nil {
		t.Fatalf("Length failed: %v", err)
	}

	if length != 5 {
		t.Errorf("Expected 5 items after reconnect, got %d", length)
	}

	// Dequeue all items
	items, err := q2.Dequeue(ctx, 10)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if len(items) != 5 {
		t.Errorf("Expected 5 items, got %d", len(items))
	}
}

func TestRedisDeadLetterQueue_AddList(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-dlq")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	dlq, err := NewRedisDeadLetterQueue(config)
	if err != nil {
		t.Fatalf("NewRedisDeadLetterQueue failed: %v", err)
	}
	defer dlq.Close()

	ctx := context.Background()

	// Add items
	item1 := map[string]string{"id": "1", "data": "test1"}
	err1 := dlq.Add(ctx, item1, ErrMaxRetriesExceeded)
	if err1 != nil {
		t.Fatalf("Add failed: %v", err1)
	}

	item2 := map[string]string{"id": "2", "data": "test2"}
	err2 := dlq.Add(ctx, item2, ErrQueueClosed)
	if err2 != nil {
		t.Fatalf("Add failed: %v", err2)
	}

	// List items
	items, err := dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// Verify error messages
	for _, item := range items {
		if item.Error == "" {
			t.Error("Expected non-empty error message")
		}
		if item.ID == "" {
			t.Error("Expected non-empty ID")
		}
	}
}

func TestRedisDeadLetterQueue_Remove(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-dlq-remove")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	dlq, err := NewRedisDeadLetterQueue(config)
	if err != nil {
		t.Fatalf("NewRedisDeadLetterQueue failed: %v", err)
	}
	defer dlq.Close()

	ctx := context.Background()

	// Add item
	item := map[string]string{"data": "test"}
	err = dlq.Add(ctx, item, ErrMaxRetriesExceeded)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// List to get ID
	items, err := dlq.List(ctx, 1)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	itemID := items[0].ID

	// Remove item
	err = dlq.Remove(ctx, itemID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify removed
	items, err = dlq.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items after removal, got %d", len(items))
	}
}

func TestRedisDeadLetterQueue_Persistence(t *testing.T) {
	if !isRedisAvailable(t) {
		return
	}

	config := DefaultConfig("test-redis-dlq-persist")
	config.UseRedis = true
	config.RedisAddr = getRedisAddr()
	config.RedisDB = 15

	cleanupRedisQueue(t, config)
	defer cleanupRedisQueue(t, config)

	ctx := context.Background()

	// Create DLQ, add items, close
	dlq1, err := NewRedisDeadLetterQueue(config)
	if err != nil {
		t.Fatalf("NewRedisDeadLetterQueue failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		err := dlq1.Add(ctx, map[string]int{"value": i}, ErrMaxRetriesExceeded)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	err = dlq1.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Create new DLQ instance - items should still be there
	dlq2, err := NewRedisDeadLetterQueue(config)
	if err != nil {
		t.Fatalf("NewRedisDeadLetterQueue failed: %v", err)
	}
	defer dlq2.Close()

	items, err := dlq2.List(ctx, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("Expected 3 items after reconnect, got %d", len(items))
	}
}
