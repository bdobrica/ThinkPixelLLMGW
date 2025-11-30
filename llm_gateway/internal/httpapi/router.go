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
	"llm_gateway/internal/queue"
	"llm_gateway/internal/ratelimit"
	"llm_gateway/internal/storage"
)

// Dependencies aggregates all services the HTTP layer needs.
type Dependencies struct {
	APIKeys       auth.APIKeyStore
	AdminStore    auth.AdminStore
	Providers     providers.Registry
	RateLimit     ratelimit.Limiter
	Billing       billing.Service
	Logger        logging.Sink
	Metrics       metrics.Metrics
	RequestLogger *logging.RequestLogger
	// Queue workers for async processing
	BillingWorker *billing.BillingQueueWorker
	UsageWorker   *storage.UsageQueueWorker
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
	adminUserRepo := storage.NewAdminUserRepository(db)
	adminTokenRepo := storage.NewAdminTokenRepository(db)

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

	// Initialize request logger
	requestLogger, err := logging.NewLogger(
		cfg.RequestLogger.FilePathTemplate,
		cfg.RequestLogger.MaxSize,
		cfg.RequestLogger.MaxFiles,
		cfg.RequestLogger.BufferSize,
		cfg.RequestLogger.FlushInterval,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize request logger: %w", err)
	}

	// Initialize queue infrastructure
	// Check if Redis is available for queues
	useRedis := redisClient != nil && cfg.Redis.Address != ""

	// Create billing queue
	var billingQueue queue.Queue
	var billingDLQ queue.DeadLetterQueue
	billingQueueCfg := queue.DefaultConfig("billing")
	billingQueueCfg.UseRedis = useRedis
	billingQueueCfg.BatchSize = 100
	billingQueueCfg.BatchTimeout = 5 * time.Second
	billingQueueCfg.MaxRetries = 3
	billingQueueCfg.RetryBackoff = 1 * time.Second

	if useRedis {
		billingQueueCfg.RedisAddr = cfg.Redis.Address
		billingQueueCfg.RedisPassword = cfg.Redis.Password
		billingQueueCfg.RedisDB = cfg.Redis.DB
		billingQueue, err = queue.NewRedisQueue(billingQueueCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create billing queue: %w", err)
		}
		billingDLQ, err = queue.NewRedisDeadLetterQueue(billingQueueCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create billing DLQ: %w", err)
		}
	} else {
		billingQueue = queue.NewMemoryQueue(billingQueueCfg)
		billingDLQ = queue.NewMemoryDeadLetterQueue()
	}

	// Create usage queue
	var usageQueue queue.Queue
	var usageDLQ queue.DeadLetterQueue
	usageQueueCfg := queue.DefaultConfig("usage")
	usageQueueCfg.UseRedis = useRedis
	usageQueueCfg.BatchSize = 100
	usageQueueCfg.BatchTimeout = 5 * time.Second
	usageQueueCfg.MaxRetries = 3
	usageQueueCfg.RetryBackoff = 1 * time.Second

	if useRedis {
		usageQueueCfg.RedisAddr = cfg.Redis.Address
		usageQueueCfg.RedisPassword = cfg.Redis.Password
		usageQueueCfg.RedisDB = cfg.Redis.DB
		usageQueue, err = queue.NewRedisQueue(usageQueueCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create usage queue: %w", err)
		}
		usageDLQ, err = queue.NewRedisDeadLetterQueue(usageQueueCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create usage DLQ: %w", err)
		}
	} else {
		usageQueue = queue.NewMemoryQueue(usageQueueCfg)
		usageDLQ = queue.NewMemoryDeadLetterQueue()
	}

	// Create queue workers
	billingWorker := billing.NewBillingQueueWorker(billingQueue, billingDLQ, billingService, billingQueueCfg)
	usageWorker := storage.NewUsageQueueWorker(usageQueue, usageDLQ, db, usageQueueCfg)

	// Start queue workers
	billingWorker.Start(context.Background())
	usageWorker.Start(context.Background())

	// Create dependencies
	deps := &Dependencies{
		APIKeys:       NewDatabaseAPIKeyStore(apiKeyRepo),
		AdminStore:    NewAdminStoreAdapter(adminUserRepo, adminTokenRepo),
		Providers:     registry,
		RateLimit:     rateLimiter,
		Billing:       billingService,
		Logger:        NewRedisLoggingSink(logBuffer),
		Metrics:       metrics.NewNoopMetrics(), // TODO: Implement Prometheus metrics
		RequestLogger: requestLogger,
		BillingWorker: billingWorker,
		UsageWorker:   usageWorker,
	}

	// Create router
	mux := http.NewServeMux()
	registerRoutes(mux, deps, cfg)

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

func registerRoutes(mux *http.ServeMux, deps *Dependencies, cfg *config.Config) {
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

	// Admin authentication endpoints - public (no middleware)
	adminAuthHandler := NewAdminAuthHandler(deps.AdminStore, cfg)
	mux.HandleFunc("/admin/auth/login", adminAuthHandler.Login)
	mux.HandleFunc("/admin/auth/token", adminAuthHandler.TokenAuth)

	// Protected admin test endpoint
	adminJWT := middleware.AdminJWTMiddleware(cfg)
	mux.Handle("/admin/test", adminJWT(http.HandlerFunc(adminAuthHandler.TestProtected)))

	// Admin management endpoints - protected with AdminJWTMiddleware
	// Require at least "viewer" role
	viewerMiddleware := middleware.AdminJWTMiddleware(cfg, "viewer")
	mux.Handle("/admin/keys", viewerMiddleware(http.HandlerFunc(deps.handleAdminKeys)))
	mux.Handle("/admin/providers", viewerMiddleware(http.HandlerFunc(deps.handleAdminProviders)))
}
