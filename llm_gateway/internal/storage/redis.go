package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis connection and provides health checks
type RedisClient struct {
	client *redis.Client
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	// Connection settings
	Address  string // host:port
	Password string
	DB       int // Database number (0-15)

	// Pool settings
	PoolSize     int
	MinIdleConns int

	// Timeouts
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Connection lifecycle
	MaxConnAge    time.Duration
	PoolTimeout   time.Duration
	IdleTimeout   time.Duration
	IdleCheckFreq time.Duration

	// Retry settings
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
}

// DefaultRedisConfig returns default Redis configuration
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Address:  "localhost:6379",
		Password: "",
		DB:       0,

		PoolSize:     10,
		MinIdleConns: 2,

		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		MaxConnAge:    30 * time.Minute,
		PoolTimeout:   4 * time.Second,
		IdleTimeout:   5 * time.Minute,
		IdleCheckFreq: 1 * time.Minute,

		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	}
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,

		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,

		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,

		MaxConnAge:      cfg.MaxConnAge,
		PoolTimeout:     cfg.PoolTimeout,
		IdleTimeout:     cfg.IdleTimeout,
		ConnMaxIdleTime: cfg.IdleTimeout,

		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Ping checks if Redis is reachable
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Health returns the health status of Redis
func (r *RedisClient) Health(ctx context.Context) error {
	// Check connection
	if err := r.Ping(ctx); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	// Check if we can execute a simple command
	if err := r.client.Set(ctx, "health_check", "ok", 1*time.Second).Err(); err != nil {
		return fmt.Errorf("redis write failed: %w", err)
	}

	val, err := r.client.Get(ctx, "health_check").Result()
	if err != nil {
		return fmt.Errorf("redis read failed: %w", err)
	}

	if val != "ok" {
		return fmt.Errorf("redis health check value mismatch")
	}

	return nil
}

// RedisStats represents Redis connection pool statistics
type RedisStats struct {
	Hits     uint32
	Misses   uint32
	Timeouts uint32

	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
}

// GetStats returns current Redis connection pool statistics
func (r *RedisClient) GetStats() RedisStats {
	stats := r.client.PoolStats()

	return RedisStats{
		Hits:     stats.Hits,
		Misses:   stats.Misses,
		Timeouts: stats.Timeouts,

		TotalConns: stats.TotalConns,
		IdleConns:  stats.IdleConns,
		StaleConns: stats.StaleConns,
	}
}

// Client returns the underlying Redis client
// Use this for custom Redis operations not covered by helper methods
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// Pipeline creates a new Redis pipeline for batching commands
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// TxPipeline creates a new Redis transaction pipeline
func (r *RedisClient) TxPipeline() redis.Pipeliner {
	return r.client.TxPipeline()
}

// ClusterClient wraps Redis cluster connection (future support)
type ClusterClient struct {
	client *redis.ClusterClient
}

// ClusterConfig holds Redis cluster configuration
type ClusterConfig struct {
	Addrs    []string // List of cluster node addresses
	Password string

	// Pool settings
	PoolSize     int
	MinIdleConns int

	// Timeouts
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Retry settings
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
}

// NewClusterClient creates a new Redis cluster client
func NewClusterClient(cfg ClusterConfig) (*ClusterClient, error) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    cfg.Addrs,
		Password: cfg.Password,

		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,

		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,

		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis cluster: %w", err)
	}

	return &ClusterClient{client: client}, nil
}

// Close closes the Redis cluster connection
func (c *ClusterClient) Close() error {
	return c.client.Close()
}

// Ping checks if Redis cluster is reachable
func (c *ClusterClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Client returns the underlying Redis cluster client
func (c *ClusterClient) Client() *redis.ClusterClient {
	return c.client
}
