package httpapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"

	"llm_gateway/internal/billing"
	"llm_gateway/internal/config"
	"llm_gateway/internal/logging"
	"llm_gateway/internal/metrics"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/ratelimit"
	"llm_gateway/internal/storage"
)

// Dependencies aggregates all services the HTTP layer needs.
type Dependencies struct {
	APIKeys   APIKeyStore
	Providers providers.Registry
	RateLimit ratelimit.Limiter
	Billing   billing.Service
	Logger    logging.Sink
	Metrics   metrics.Metrics
}

// APIKeyStore resolves plaintext API keys into stored records
type APIKeyStore interface {
	Lookup(ctx context.Context, plaintextKey string) (*models.APIKey, error)
}

// NewRouter creates an HTTP router with all dependencies wired up
func NewRouter(cfg *config.Config) (*http.ServeMux, *Dependencies, error) {
	// Initialize database
	dbConfig := storage.DBConfig{
		Host:            cfg.Database.URL, // This is actually the full URL for now
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
		APIKeyCacheSize: cfg.Cache.APIKeyCacheSize,
		APIKeyCacheTTL:  cfg.Cache.APIKeyCacheTTL,
		ModelCacheSize:  cfg.Cache.ModelCacheSize,
		ModelCacheTTL:   cfg.Cache.ModelCacheTTL,
	}

	db, err := storage.NewDB(dbConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis client
	redisClient, err := storage.NewRedisClient(storage.RedisConfig{
		Address:         cfg.Redis.Address,
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.DB,
		PoolSize:        cfg.Redis.PoolSize,
		MinIdleConns:    cfg.Redis.MinIdleConns,
		DialTimeout:     cfg.Redis.DialTimeout,
		ReadTimeout:     cfg.Redis.ReadTimeout,
		WriteTimeout:    cfg.Redis.WriteTimeout,
		MaxConnAge:      cfg.Redis.MaxConnAge,
		PoolTimeout:     cfg.Redis.PoolTimeout,
		IdleTimeout:     cfg.Redis.IdleTimeout,
		MaxRetries:      cfg.Redis.MaxRetries,
		MinRetryBackoff: cfg.Redis.MinRetryBackoff,
		MaxRetryBackoff: cfg.Redis.MaxRetryBackoff,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	// Initialize repositories
	apiKeyRepo := storage.NewAPIKeyRepository(db)

	// Initialize encryption for provider credentials
	// Use encryption key from config
	encryptionKeyHex := cfg.EncryptionKey
	if encryptionKeyHex == "" {
		encryptionKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // Default for dev
	}

	// Validate it's valid hex and 64 chars (32 bytes)
	if len(encryptionKeyHex) != 64 {
		return nil, nil, fmt.Errorf("encryption key must be 64 hex characters (32 bytes)")
	}
	if _, err := hex.DecodeString(encryptionKeyHex); err != nil {
		return nil, nil, fmt.Errorf("encryption key must be valid hex: %w", err)
	}

	encryption, err := storage.NewEncryption(encryptionKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize encryption: %w", err)
	}

	// Initialize provider registry
	registry, err := providers.NewProviderRegistry(providers.RegistryConfig{
		DB:             db,
		Encryption:     encryption,
		ReloadInterval: cfg.Provider.ReloadInterval,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize provider registry: %w", err)
	}

	// Initialize rate limiter
	rateLimiter := ratelimit.NewRateLimiter(redisClient.Client())

	// Initialize billing service
	billingService := billing.NewRedisBillingService(
		redisClient.Client(),
		db,
		cfg.Billing.SyncInterval,
	)

	// Initialize logging buffer
	logBuffer := logging.NewRedisBuffer(redisClient.Client(), logging.RedisBufferConfig{
		QueueKey:  "gateway:logs",
		MaxSize:   cfg.Logging.BufferMaxSize,
		BatchSize: 1000,
	})

	// Create dependencies
	deps := &Dependencies{
		APIKeys:   NewDatabaseAPIKeyStore(apiKeyRepo),
		Providers: registry,
		RateLimit: rateLimiter,
		Billing:   billingService,
		Logger:    NewRedisLoggingSink(logBuffer),
		Metrics:   metrics.NewNoopMetrics(), // TODO: Implement Prometheus metrics
	}

	// Create router
	mux := http.NewServeMux()
	registerRoutes(mux, deps)

	return mux, deps, nil
}

func registerRoutes(mux *http.ServeMux, deps *Dependencies) {
	// Public OpenAI-compatible proxy endpoint
	mux.HandleFunc("/v1/chat/completions", deps.handleChat)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint
	mux.Handle("/metrics", deps.Metrics.HTTPHandler())

	// TODO: Add admin endpoints when JWT auth is implemented
	// mux.HandleFunc("/admin/keys", deps.withJWTAuth(deps.handleAdminKeys))
	// mux.HandleFunc("/admin/providers", deps.withJWTAuth(deps.handleAdminProviders))
}
