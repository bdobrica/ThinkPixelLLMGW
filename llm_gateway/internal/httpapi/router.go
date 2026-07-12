package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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
	RateLimit     ratelimit.LimiterWithDetails
	Billing       billing.Service
	Logger        logging.Sink
	Metrics       metrics.Metrics
	RequestLogger *logging.RequestLogger
	// Queue workers for async processing
	BillingWorker billingUpdateEnqueuer
	UsageWorker   usageRecordEnqueuer
	// Database and encryption for admin handlers
	DB         *storage.DB
	Encryption *storage.Encryption

	redisClient   *storage.RedisClient
	billingQueue  queue.Queue
	billingDLQ    queue.DeadLetterQueue
	usageQueue    queue.Queue
	usageDLQ      queue.DeadLetterQueue
	billingWorker *billing.BillingQueueWorker
	usageWorker   *storage.UsageQueueWorker
	workerCancel  context.CancelFunc
	closeOnce     sync.Once
	closeErr      error
}

// Close stops background producers and workers, flushes buffered state, and
// releases every infrastructure resource owned by NewRouter. It is idempotent.
func (d *Dependencies) Close(ctx context.Context) error {
	d.closeOnce.Do(func() {
		var errs []error
		if d.workerCancel != nil {
			d.workerCancel()
		}
		if d.billingWorker != nil {
			errs = appendCloseError(errs, "billing worker", d.billingWorker.StopContext(ctx))
		}
		if d.usageWorker != nil {
			errs = appendCloseError(errs, "usage worker", d.usageWorker.StopContext(ctx))
		}
		if d.RequestLogger != nil {
			d.RequestLogger.Shutdown()
		}
		if d.Logger != nil {
			errs = appendCloseError(errs, "logging sink", d.Logger.Shutdown(ctx))
		}
		if service, ok := d.Billing.(interface{ Shutdown(context.Context) error }); ok {
			errs = appendCloseError(errs, "billing service", service.Shutdown(ctx))
		}
		if registry, ok := d.Providers.(interface{ Close() error }); ok {
			errs = appendCloseError(errs, "provider registry", registry.Close())
		}
		if d.billingQueue != nil {
			errs = appendCloseError(errs, "billing queue", d.billingQueue.Close())
		}
		if d.billingDLQ != nil {
			errs = appendCloseError(errs, "billing dead-letter queue", d.billingDLQ.Close())
		}
		if d.usageQueue != nil {
			errs = appendCloseError(errs, "usage queue", d.usageQueue.Close())
		}
		if d.usageDLQ != nil {
			errs = appendCloseError(errs, "usage dead-letter queue", d.usageDLQ.Close())
		}
		if d.redisClient != nil {
			errs = appendCloseError(errs, "Redis client", d.redisClient.Close())
		}
		if d.DB != nil {
			errs = appendCloseError(errs, "database", d.DB.Close())
		}
		d.closeErr = errors.Join(errs...)
	})
	return d.closeErr
}

func appendCloseError(errs []error, name string, err error) []error {
	if err != nil {
		return append(errs, fmt.Errorf("close %s: %w", name, err))
	}
	return errs
}

// NewRouter creates an HTTP router with all dependencies wired up
func NewRouter(cfg *config.Config) (_ *http.ServeMux, deps *Dependencies, err error) {
	deps = &Dependencies{}
	defer func() {
		if err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if cleanupErr := deps.Close(cleanupCtx); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("rollback router dependencies: %w", cleanupErr))
			}
		}
	}()
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
	deps.DB = db

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
	deps.redisClient = redisClient

	// Initialize repositories
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	adminUserRepo := storage.NewAdminUserRepository(db)
	adminTokenRepo := storage.NewAdminTokenRepository(db)

	// Initialize encryption for provider credentials
	encryptionKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encryptionKeyHex == "" {
		return nil, nil, fmt.Errorf("ENCRYPTION_KEY is required")
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
		RequestTimeout: cfg.Provider.RequestTimeout,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize provider registry: %w", err)
	}
	deps.Providers = registry

	// Initialize rate limiter
	rateLimiter := ratelimit.NewRateLimiter(redisClient.Client())

	// Initialize billing service
	billingService := billing.NewRedisBillingService(
		redisClient.Client(),
		db,
		5*time.Minute, // Sync to database every 5 minutes
	)
	deps.Billing = billingService

	// Initialize logging buffer
	logBuffer := logging.NewRedisBuffer(redisClient.Client(), logging.RedisBufferConfig{
		QueueKey:  "gateway:logs",
		MaxSize:   100000, // 100K max entries
		BatchSize: 1000,
	})

	// Initialize S3 logging sink
	s3SinkConfig := logging.S3SinkConfig{
		Enabled:       cfg.LoggingSink.Enabled,
		BufferSize:    cfg.LoggingSink.BufferSize,
		FlushSize:     cfg.LoggingSink.FlushSize,
		FlushInterval: cfg.LoggingSink.FlushInterval,
		S3Bucket:      cfg.LoggingSink.S3Bucket,
		S3Region:      cfg.LoggingSink.S3Region,
		S3Prefix:      cfg.LoggingSink.S3Prefix,
		PodName:       cfg.LoggingSink.PodName,
	}
	s3Sink, err := logging.NewSinkFromConfig(context.Background(), s3SinkConfig, logBuffer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize S3 sink: %w", err)
	}
	deps.Logger = s3Sink

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
	deps.RequestLogger = requestLogger

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
		deps.billingQueue = billingQueue
		billingDLQ, err = queue.NewRedisDeadLetterQueue(billingQueueCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create billing DLQ: %w", err)
		}
		deps.billingDLQ = billingDLQ
	} else {
		billingQueue = queue.NewMemoryQueue(billingQueueCfg)
		billingDLQ = queue.NewMemoryDeadLetterQueue()
	}
	deps.billingQueue, deps.billingDLQ = billingQueue, billingDLQ

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
		deps.usageQueue = usageQueue
		usageDLQ, err = queue.NewRedisDeadLetterQueue(usageQueueCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create usage DLQ: %w", err)
		}
	} else {
		usageQueue = queue.NewMemoryQueue(usageQueueCfg)
		usageDLQ = queue.NewMemoryDeadLetterQueue()
	}
	deps.usageQueue, deps.usageDLQ = usageQueue, usageDLQ

	// Create queue workers
	billingWorker := billing.NewBillingQueueWorker(billingQueue, billingDLQ, billingService, billingQueueCfg)
	usageWorker := storage.NewUsageQueueWorker(usageQueue, usageDLQ, db, usageQueueCfg)

	// Start queue workers
	workerCtx, workerCancel := context.WithCancel(context.Background())
	deps.workerCancel = workerCancel
	billingWorker.Start(workerCtx)
	usageWorker.Start(workerCtx)

	// Initialize Prometheus metrics
	prometheusMetrics := metrics.NewPrometheusMetrics()

	// Create dependencies
	deps.APIKeys = NewDatabaseAPIKeyStore(apiKeyRepo)
	deps.AdminStore = NewAdminStoreAdapter(adminUserRepo, adminTokenRepo)
	deps.RateLimit = rateLimiter
	deps.Metrics = prometheusMetrics
	deps.BillingWorker = billingWorker
	deps.UsageWorker = usageWorker
	deps.Encryption = encryption
	deps.billingWorker = billingWorker
	deps.usageWorker = usageWorker
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
	mux.Handle("/ready", readinessHandler(deps, cfg.HTTPServer.ReadinessTimeout))

	// Metrics endpoint - protected with a dedicated scrape token.
	mux.Handle("/metrics", bearerTokenAuth(cfg.MetricsAuthToken, deps.Metrics.HTTPHandler()))

	// Admin authentication endpoints - public (no middleware)
	adminAuthHandler := NewAdminAuthHandler(deps.AdminStore, cfg)
	mux.HandleFunc("/admin/auth/login", adminAuthHandler.Login)
	mux.HandleFunc("/admin/auth/token", adminAuthHandler.TokenAuth)

	// Protected admin test endpoint
	adminJWT := middleware.AdminJWTMiddleware(cfg)
	mux.Handle("/admin/test", adminJWT(http.HandlerFunc(adminAuthHandler.TestProtected)))

	// Admin management endpoints - protected with AdminJWTMiddleware
	// Require at least "viewer" role
	viewerMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleViewer.String())
	// Admin role required for create, update, delete operations
	adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())

	// API Key management endpoints
	adminAPIKeysHandler := NewAdminAPIKeysHandler(deps.DB)
	mux.Handle("/admin/keys", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// List API keys - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminAPIKeysHandler.List)).ServeHTTP(w, r)
		case http.MethodPost:
			// Create API key - admin role required
			adminMiddleware(http.HandlerFunc(adminAPIKeysHandler.Create)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// API Key detail endpoints with ID
	mux.Handle("/admin/keys/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a regenerate request
		if strings.HasSuffix(r.URL.Path, "/regenerate") && r.Method == http.MethodPost {
			// Regenerate API key - admin role required
			adminMiddleware(http.HandlerFunc(adminAPIKeysHandler.Regenerate)).ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// Get API key details - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminAPIKeysHandler.GetByID)).ServeHTTP(w, r)
		case http.MethodPut:
			// Update API key - admin role required
			adminMiddleware(http.HandlerFunc(adminAPIKeysHandler.Update)).ServeHTTP(w, r)
		case http.MethodDelete:
			// Revoke API key - admin role required
			adminMiddleware(http.HandlerFunc(adminAPIKeysHandler.Delete)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Provider management endpoints
	adminProvidersHandler := NewAdminProvidersHandler(deps.DB, deps.Encryption, deps.Providers)
	mux.Handle("/admin/providers", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// List providers - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminProvidersHandler.List)).ServeHTTP(w, r)
		case http.MethodPost:
			// Create provider - admin role required
			adminMiddleware(http.HandlerFunc(adminProvidersHandler.Create)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Provider detail endpoints with ID
	mux.Handle("/admin/providers/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Get provider details - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminProvidersHandler.GetByID)).ServeHTTP(w, r)
		case http.MethodPut:
			// Update provider - admin role required
			adminMiddleware(http.HandlerFunc(adminProvidersHandler.Update)).ServeHTTP(w, r)
		case http.MethodDelete:
			// Disable provider - admin role required
			adminMiddleware(http.HandlerFunc(adminProvidersHandler.Delete)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model management endpoints
	adminModelsHandler := NewAdminModelsHandler(deps.DB, deps.Providers)
	mux.Handle("/admin/models", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// List models - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminModelsHandler.List)).ServeHTTP(w, r)
		case http.MethodPost:
			// Create model - admin role required
			adminMiddleware(http.HandlerFunc(adminModelsHandler.Create)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model detail endpoints with ID
	mux.Handle("/admin/models/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Get model details - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminModelsHandler.GetByID)).ServeHTTP(w, r)
		case http.MethodPut:
			// Update model - admin role required
			adminMiddleware(http.HandlerFunc(adminModelsHandler.Update)).ServeHTTP(w, r)
		case http.MethodDelete:
			// Delete model - admin role required
			adminMiddleware(http.HandlerFunc(adminModelsHandler.Delete)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Model Alias management endpoints
	adminAliasesHandler := NewAdminAliasesHandler(deps.DB, deps.Providers)
	mux.Handle("/admin/aliases", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// List aliases - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminAliasesHandler.List)).ServeHTTP(w, r)
		case http.MethodPost:
			// Create alias - admin role required
			adminMiddleware(http.HandlerFunc(adminAliasesHandler.Create)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Alias detail endpoints with ID
	mux.Handle("/admin/aliases/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Get alias details - viewer role sufficient
			viewerMiddleware(http.HandlerFunc(adminAliasesHandler.GetByID)).ServeHTTP(w, r)
		case http.MethodPut:
			// Update alias - admin role required
			adminMiddleware(http.HandlerFunc(adminAliasesHandler.Update)).ServeHTTP(w, r)
		case http.MethodDelete:
			// Delete alias - admin role required
			adminMiddleware(http.HandlerFunc(adminAliasesHandler.Delete)).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}

func bearerTokenAuth(token string, next http.Handler) http.Handler {
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type readinessCheck struct {
	name string
	run  func(context.Context) error
}

func readinessHandler(deps *Dependencies, timeout time.Duration) http.Handler {
	checks := []readinessCheck{
		{"database", deps.DB.Health},
		{"redis", deps.redisClient.Health},
		{"providers", func(ctx context.Context) error { _, err := deps.Providers.ListProviders(ctx); return err }},
		{"billing worker", func(context.Context) error {
			if !deps.billingWorker.Ready() {
				return errors.New("not running")
			}
			return nil
		}},
		{"usage worker", func(context.Context) error {
			if !deps.usageWorker.Ready() {
				return errors.New("not running")
			}
			return nil
		}},
	}
	return readinessHandlerForChecks(checks, timeout, deps.Metrics)
}

func readinessHandlerForChecks(checks []readinessCheck, timeout time.Duration, metric metrics.Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		results := make(chan error, len(checks))
		for _, check := range checks {
			go func(c readinessCheck) { results <- c.run(ctx) }(check)
		}
		ready := true
		for range checks {
			select {
			case err := <-results:
				if err != nil {
					ready = false
				}
			case <-ctx.Done():
				ready = false
				metric.RecordReadiness(false)
				writeReadiness(w, false)
				return
			}
		}
		metric.RecordReadiness(ready)
		writeReadiness(w, ready)
	})
}

func writeReadiness(w http.ResponseWriter, ready bool) {
	w.Header().Set("Content-Type", "application/json")
	status, code := "ready", http.StatusOK
	if !ready {
		status, code = "unavailable", http.StatusServiceUnavailable
	}
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
