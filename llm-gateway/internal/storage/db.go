package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB wraps the database connection and provides health checks
type DB struct {
	conn *sqlx.DB

	// Cache for frequently accessed data
	apiKeyCache *LRUCache
	modelCache  *LRUCache
}

// DBConfig holds database configuration
type DBConfig struct {
	// Connection settings
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string

	// Pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Query timeouts
	QueryTimeout time.Duration

	// Cache settings
	APIKeyCacheSize int
	APIKeyCacheTTL  time.Duration
	ModelCacheSize  int
	ModelCacheTTL   time.Duration
}

// DefaultDBConfig returns default database configuration
func DefaultDBConfig() DBConfig {
	return DBConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "llmgateway",
		User:     "postgres",
		Password: "",
		SSLMode:  "disable",

		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,

		QueryTimeout: 5 * time.Second,

		APIKeyCacheSize: 1000,
		APIKeyCacheTTL:  5 * time.Minute,
		ModelCacheSize:  500,
		ModelCacheTTL:   15 * time.Minute,
	}
}

// NewDB creates a new database connection with caching
func NewDB(cfg DBConfig) (*DB, error) {
	// Build connection string
	dsn := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password, cfg.SSLMode,
	)

	// Connect to database
	conn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(cfg.MaxOpenConns)
	conn.SetMaxIdleConns(cfg.MaxIdleConns)
	conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	conn.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	db := &DB{
		conn:        conn,
		apiKeyCache: NewLRUCache(cfg.APIKeyCacheSize, cfg.APIKeyCacheTTL),
		modelCache:  NewLRUCache(cfg.ModelCacheSize, cfg.ModelCacheTTL),
	}

	return db, nil
}

// Close closes the database connection and clears caches
func (db *DB) Close() error {
	db.apiKeyCache.Clear()
	db.modelCache.Clear()
	return db.conn.Close()
}

// Ping checks if the database is reachable
func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// Health returns the health status of the database
func (db *DB) Health(ctx context.Context) error {
	// Check connection
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if we can execute a simple query
	var result int
	err := db.conn.GetContext(ctx, &result, "SELECT 1")
	if err != nil {
		return fmt.Errorf("health check query failed: %w", err)
	}

	return nil
}

// Stats returns database statistics
type DBStats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64

	APIKeyCacheStats CacheStats
	ModelCacheStats  CacheStats
}

// GetStats returns current database and cache statistics
func (db *DB) GetStats() DBStats {
	stats := db.conn.Stats()

	return DBStats{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration,
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,

		APIKeyCacheStats: db.apiKeyCache.GetStats(),
		ModelCacheStats:  db.modelCache.GetStats(),
	}
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return db.conn.BeginTxx(ctx, opts)
}

// Conn returns the underlying sqlx connection
// Use this for custom queries not covered by repositories
func (db *DB) Conn() *sqlx.DB {
	return db.conn
}

// GetAPIKeyCache returns the API key cache
func (db *DB) GetAPIKeyCache() *LRUCache {
	return db.apiKeyCache
}

// GetModelCache returns the model cache
func (db *DB) GetModelCache() *LRUCache {
	return db.modelCache
}

// CleanupExpiredCacheEntries removes expired entries from all caches
// Should be called periodically (e.g., every minute)
func (db *DB) CleanupExpiredCacheEntries() (apiKeyRemoved, modelRemoved int) {
	apiKeyRemoved = db.apiKeyCache.CleanupExpired()
	modelRemoved = db.modelCache.CleanupExpired()
	return
}

// Repository factory methods

// NewAPIKeyRepository creates a new API key repository
func (db *DB) NewAPIKeyRepository() *APIKeyRepository {
	return NewAPIKeyRepository(db)
}

// NewModelRepository creates a new model repository
func (db *DB) NewModelRepository() *ModelRepository {
	return NewModelRepository(db)
}

// NewProviderRepository creates a new provider repository
func (db *DB) NewProviderRepository() *ProviderRepository {
	return NewProviderRepository(db)
}

// NewUsageRepository creates a new usage repository
func (db *DB) NewUsageRepository() *UsageRepository {
	return NewUsageRepository(db)
}

// NewMonthlyUsageSummaryRepository creates a new monthly usage summary repository
func (db *DB) NewMonthlyUsageSummaryRepository() *MonthlyUsageSummaryRepository {
	return NewMonthlyUsageSummaryRepository(db)
}
