package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds configuration for the gateway.
type Config struct {
	HTTPPort         string
	HTTPServer       HTTPServerConfig
	JWTSecret        []byte
	MetricsAuthToken string
	Database         DatabaseConfig
	Cache            CacheConfig
	Redis            RedisConfig
	Provider         ProviderConfig
	RequestLogger    RequestLoggerConfig
	LoggingSink      LoggingSinkConfig
	Responses        ResponsesConfig
}

type HTTPServerConfig struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	ReadinessTimeout  time.Duration
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// CacheConfig holds cache settings
type CacheConfig struct {
	APIKeyCacheSize int
	APIKeyCacheTTL  time.Duration
	ModelCacheSize  int
	ModelCacheTTL   time.Duration
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Address      string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// ProviderConfig holds provider-related settings
type ProviderConfig struct {
	ReloadInterval time.Duration // How often to reload providers from database
	RequestTimeout time.Duration // Default timeout for provider requests
}

type RequestLoggerConfig struct {
	FilePathTemplate string
	MaxSize          int64
	MaxFiles         int
	BufferSize       int
	FlushInterval    time.Duration
	BodyMode         string
	MaxBodyBytes     int
	SampleRate       float64
	SensitiveFields  []string
}

// LoggingSinkConfig holds configuration for the S3-based logging sink
type LoggingSinkConfig struct {
	Enabled       bool          // Whether to enable S3 logging
	BufferSize    int           // In-memory queue size
	FlushSize     int           // Flush to S3 after this many records
	FlushInterval time.Duration // Flush to S3 after this duration
	S3Bucket      string        // S3 bucket name
	S3Region      string        // AWS region
	S3Prefix      string        // Prefix for S3 keys (e.g., "logs/")
	PodName       string        // Pod identifier for multi-pod deployments
}

type ResponsesConfig struct {
	Retention          time.Duration
	TransientRetention time.Duration
	OrphanedAfter      time.Duration
	MaxChainDepth      int
	MaxChainItems      int
	MaxChainBytes      int
	MaxChainTokens     int
}

func getEnvInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}

	return intVal
}

func getEnvInt64(key string, defaultValue int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	intVal, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return intVal
}

func getEnvFloat(key string, defaultValue float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(val)
	if err != nil {
		return defaultValue
	}

	return duration
}

func getEnvString(key string, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func requiredPositiveDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

// Load reads configuration from environment variables (and, later, other sources).
func Load() (*Config, error) {
	port := getEnvString("GATEWAY_HTTP_PORT", getEnvString("HTTP_PORT", "8080"))
	jwtSecretRaw := os.Getenv("JWT_SECRET")
	if jwtSecretRaw == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(jwtSecretRaw) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	jwtSecret := []byte(jwtSecretRaw)
	metricsAuthToken := os.Getenv("METRICS_AUTH_TOKEN")
	if len(metricsAuthToken) < 32 {
		return nil, fmt.Errorf("METRICS_AUTH_TOKEN must be at least 32 characters")
	}
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	decodedEncryptionKey, decodeErr := hex.DecodeString(encryptionKey)
	if decodeErr != nil || len(decodedEncryptionKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 64 hexadecimal characters")
	}

	// Load database configuration
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	readHeaderTimeout, err := requiredPositiveDuration("HTTP_READ_HEADER_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	readTimeout, err := requiredPositiveDuration("HTTP_READ_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	writeTimeout, err := requiredPositiveDuration("HTTP_WRITE_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	idleTimeout, err := requiredPositiveDuration("HTTP_IDLE_TIMEOUT", 120*time.Second)
	if err != nil {
		return nil, err
	}
	shutdownTimeout, err := requiredPositiveDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	readinessTimeout, err := requiredPositiveDuration("HTTP_READINESS_TIMEOUT", 2*time.Second)
	if err != nil {
		return nil, err
	}
	responseRetention, err := requiredPositiveDuration("RESPONSES_RETENTION", 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	responseTransientRetention, err := requiredPositiveDuration("RESPONSES_TRANSIENT_RETENTION", time.Hour)
	if err != nil {
		return nil, err
	}
	responseOrphanedAfter, err := requiredPositiveDuration("RESPONSES_ORPHANED_AFTER", 15*time.Minute)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		HTTPPort:         port,
		JWTSecret:        jwtSecret,
		MetricsAuthToken: metricsAuthToken,
		HTTPServer: HTTPServerConfig{
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
			ShutdownTimeout:   shutdownTimeout,
			ReadinessTimeout:  readinessTimeout,
		},
		Database: DatabaseConfig{
			URL:             dbURL,
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
		},
		Cache: CacheConfig{
			APIKeyCacheSize: getEnvInt("CACHE_API_KEY_SIZE", 1000),
			APIKeyCacheTTL:  getEnvDuration("CACHE_API_KEY_TTL", 5*time.Minute),
			ModelCacheSize:  getEnvInt("CACHE_MODEL_SIZE", 500),
			ModelCacheTTL:   getEnvDuration("CACHE_MODEL_TTL", 15*time.Minute),
		},
		Redis: RedisConfig{
			Address:      getEnvString("REDIS_ADDRESS", "localhost:6379"),
			Password:     getEnvString("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 2),
			DialTimeout:  getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		Provider: ProviderConfig{
			ReloadInterval: getEnvDuration("PROVIDER_RELOAD_INTERVAL", 5*time.Minute),
			RequestTimeout: getEnvDuration("PROVIDER_REQUEST_TIMEOUT", 60*time.Second),
		},
		RequestLogger: RequestLoggerConfig{
			FilePathTemplate: getEnvString("REQUEST_LOGGER_FILE_PATH_TEMPLATE", "/var/log/llm-gateway/requests-%s.jsonl"),
			MaxSize:          getEnvInt64("REQUEST_LOGGER_MAX_SIZE", 10_485_760),              // default 10 MB
			MaxFiles:         getEnvInt("REQUEST_LOGGER_MAX_FILES", 5),                        // default 5
			BufferSize:       getEnvInt("REQUEST_LOGGER_BUFFER_SIZE", 100),                    // default 100
			FlushInterval:    getEnvDuration("REQUEST_LOGGER_FLUSH_INTERVAL", 60*time.Second), // default 60 seconds
			BodyMode:         getEnvString("AUDIT_BODY_MODE", "hash"),
			MaxBodyBytes:     getEnvInt("AUDIT_MAX_BODY_BYTES", 4096),
			SampleRate:       getEnvFloat("AUDIT_SAMPLE_RATE", 1),
			SensitiveFields:  strings.Split(getEnvString("AUDIT_SENSITIVE_FIELDS", ""), ","),
		},
		LoggingSink: LoggingSinkConfig{
			Enabled:       getEnvString("LOGGING_SINK_ENABLED", "false") == "true",
			BufferSize:    getEnvInt("LOGGING_SINK_BUFFER_SIZE", 10000),
			FlushSize:     getEnvInt("LOGGING_SINK_FLUSH_SIZE", 1000),
			FlushInterval: getEnvDuration("LOGGING_SINK_FLUSH_INTERVAL", 5*time.Minute),
			S3Bucket:      getEnvString("LOGGING_SINK_S3_BUCKET", ""),
			S3Region:      getEnvString("LOGGING_SINK_S3_REGION", "us-east-1"),
			S3Prefix:      getEnvString("LOGGING_SINK_S3_PREFIX", "logs/"),
			PodName:       getEnvString("POD_NAME", "gateway-0"),
		},
		Responses: ResponsesConfig{
			Retention:          responseRetention,
			TransientRetention: responseTransientRetention,
			OrphanedAfter:      responseOrphanedAfter,
			MaxChainDepth:      getEnvInt("RESPONSES_MAX_CHAIN_DEPTH", 64),
			MaxChainItems:      getEnvInt("RESPONSES_MAX_CHAIN_ITEMS", 4096),
			MaxChainBytes:      getEnvInt("RESPONSES_MAX_CHAIN_BYTES", 16<<20),
			MaxChainTokens:     getEnvInt("RESPONSES_MAX_CHAIN_TOKENS", 128000),
		},
	}
	if cfg.RequestLogger.BodyMode != "none" && cfg.RequestLogger.BodyMode != "hash" && cfg.RequestLogger.BodyMode != "redacted" {
		return nil, fmt.Errorf("AUDIT_BODY_MODE must be none, hash, or redacted")
	}
	if cfg.RequestLogger.MaxBodyBytes <= 0 {
		return nil, fmt.Errorf("AUDIT_MAX_BODY_BYTES must be positive")
	}
	if cfg.RequestLogger.SampleRate < 0 || cfg.RequestLogger.SampleRate > 1 {
		return nil, fmt.Errorf("AUDIT_SAMPLE_RATE must be between 0 and 1")
	}
	if cfg.LoggingSink.Enabled {
		if cfg.LoggingSink.S3Bucket == "" {
			return nil, fmt.Errorf("LOGGING_SINK_S3_BUCKET is required when LOGGING_SINK_ENABLED=true")
		}
		if cfg.LoggingSink.S3Region == "" {
			return nil, fmt.Errorf("LOGGING_SINK_S3_REGION is required when LOGGING_SINK_ENABLED=true")
		}
		if cfg.LoggingSink.BufferSize <= 0 || cfg.LoggingSink.FlushSize <= 0 || cfg.LoggingSink.FlushInterval <= 0 {
			return nil, fmt.Errorf("logging sink buffer, flush size, and flush interval must be positive")
		}
	}
	if cfg.Responses.MaxChainDepth <= 0 || cfg.Responses.MaxChainItems <= 0 || cfg.Responses.MaxChainBytes <= 0 || cfg.Responses.MaxChainTokens <= 0 {
		return nil, fmt.Errorf("Responses chain limits must be positive")
	}

	return cfg, nil
}
