package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gateway/internal/billing"
	"gateway/internal/config"
	"gateway/internal/logging"
	"gateway/internal/metrics"
	"gateway/internal/models"
	"gateway/internal/providers"
	"gateway/internal/ratelimit"
	"gateway/internal/storage"
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
	redisClient, err := storage.NewRedisClient(cfg.Redis.Address, storage.RedisOptions{
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}
	
	// Initialize repositories
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	
	// Initialize encryption for provider credentials
	// TODO: Load encryption key from environment/config
	// For now, use a placeholder (THIS MUST BE CHANGED IN PRODUCTION)
	encryptionKey := []byte("CHANGE_THIS_32_BYTE_SECRET_KEY!")
	if len(encryptionKey) != 32 {
		return nil, nil, fmt.Errorf("encryption key must be exactly 32 bytes")
	}
	encryption, err := storage.NewEncryption(encryptionKey)
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
	rateLimiter := ratelimit.NewRateLimiter(redisClient, ratelimit.RateLimiterConfig{
		Window:     60 * time.Second, // 1 minute window
		MaxRetries: 3,
	})
	
	// Initialize billing service
	billingService := billing.NewRedisBillingService(billing.RedisBillingConfig{
		Redis:        redisClient,
		DB:           db,
		SyncInterval: 5 * time.Minute, // Sync to database every 5 minutes
	})
	
	// Initialize logging buffer
	logBuffer := logging.NewRedisBuffer(redisClient, "gateway:logs", 100000) // 100K max entries
	
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

