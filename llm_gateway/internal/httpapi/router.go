package httpapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/billing"
	"llm_gateway/internal/config"
	"llm_gateway/internal/logging"
	"llm_gateway/internal/metrics"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/ratelimit"
	"llm_gateway/internal/storage"
)

// Dependencies aggregates all services the HTTP layer needs.
type Dependencies struct {
	APIKeys   auth.APIKeyStore
	Providers providers.Registry
	RateLimit ratelimit.Limiter
	Billing   billing.Service
	Logger    logging.Sink
	Metrics   metrics.Metrics
}

// NewRouter creates an HTTP router with all dependencies wired up
func NewRouter(cfg *config.Config) (*http.ServeMux, *Dependencies, error) {
	// Initialize database
	dbConfig := storage.DBConfig{
		DSN:             cfg.Database.URL,
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
		Address:      cfg.Redis.Address,
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
	// TODO: Load encryption key from environment variable
	// For now, use a placeholder (THIS MUST BE CHANGED IN PRODUCTION)
	encryptionKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encryptionKeyHex == "" {
		encryptionKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // Default for dev
	}

	// Validate it's valid hex and 64 chars (32 bytes)
	if len(encryptionKeyHex) != 64 {
		return nil, nil, fmt.Errorf("encryption key must be 64 hex characters (32 bytes)")
	}
	encryptionKeyBytes, err := hex.DecodeString(encryptionKeyHex)
	if err != nil {
		return nil, nil, fmt.Errorf("encryption key must be valid hex: %w", err)
	}

	encryption, err := storage.NewEncryption(encryptionKeyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize encryption: %w", err)
	}

	// Update provider credentials from environment variables before loading registry
	if err := updateProviderCredentialsFromEnv(context.Background(), db, encryption); err != nil {
		return nil, nil, fmt.Errorf("failed to update provider credentials: %w", err)
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
	// TODO: Create a wrapper that implements the Limiter interface with per-API-key limits
	rateLimiter := ratelimit.NewNoopLimiter()

	// Initialize billing service
	billingService := billing.NewRedisBillingService(
		redisClient.Client(),
		db,
		5*time.Minute, // Sync to database every 5 minutes
	)

	// Initialize logging buffer
	logBuffer := logging.NewRedisBuffer(redisClient.Client(), logging.RedisBufferConfig{
		QueueKey:  "gateway:logs",
		MaxSize:   100000, // 100K max entries
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

// updateProviderCredentialsFromEnv updates provider credentials from environment variables
func updateProviderCredentialsFromEnv(ctx context.Context, db *storage.DB, encryption *storage.Encryption) error {
	providerRepo := storage.NewProviderRepository(db)

	// Map of environment variable name to provider name and credential key
	envMapping := map[string]struct {
		providerName string
		credKey      string
	}{
		"OPENAI_API_KEY":    {"openai", "api_key"},
		"ANTHROPIC_API_KEY": {"anthropic", "api_key"},
		// Add more providers as needed
	}

	for envVar, mapping := range envMapping {
		apiKey := os.Getenv(envVar)
		if apiKey == "" {
			continue // Skip if not set
		}

		// Get provider by name
		provider, err := providerRepo.GetByName(ctx, mapping.providerName)
		if err != nil {
			// Provider doesn't exist, skip
			continue
		}

		// Encrypt the API key
		encryptedKey, err := encryption.Encrypt([]byte(apiKey))
		if err != nil {
			return fmt.Errorf("failed to encrypt %s: %w", envVar, err)
		}

		// Update credentials
		if provider.EncryptedCredentials == nil {
			provider.EncryptedCredentials = make(map[string]interface{})
		}
		provider.EncryptedCredentials[mapping.credKey] = encryptedKey

		// Save to database
		if err := providerRepo.Update(ctx, provider); err != nil {
			return fmt.Errorf("failed to update provider %s: %w", mapping.providerName, err)
		}
	}

	return nil
}

func registerRoutes(mux *http.ServeMux, deps *Dependencies) {
	// OpenAI-compatible proxy endpoint - protected with API key middleware
	apiKeyMiddleware := middleware.APIKeyMiddleware(deps.APIKeys)
	mux.Handle("/v1/chat/completions", apiKeyMiddleware(http.HandlerFunc(deps.handleChat)))

	// Health check endpoint - public
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics endpoint - public
	mux.Handle("/metrics", deps.Metrics.HTTPHandler())

	// Admin endpoints - protected with JWT middleware
	jwtMiddleware := middleware.JWTMiddleware(deps.APIKeys)
	mux.Handle("/admin/keys", jwtMiddleware(http.HandlerFunc(deps.handleAdminKeys)))
	mux.Handle("/admin/providers", jwtMiddleware(http.HandlerFunc(deps.handleAdminProviders)))
}
