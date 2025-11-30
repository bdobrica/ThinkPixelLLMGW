package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the gateway.
type Config struct {
	HTTPPort      string
	JWTSecret     []byte
	Database      DatabaseConfig
	Cache         CacheConfig
	Redis         RedisConfig
	Provider      ProviderConfig
	RequestLogger RequestLoggerConfig
	LoggingSink   LoggingSinkConfig
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

// Load reads configuration from environment variables (and, later, other sources).
func Load() (*Config, error) {
	port := getEnvString("HTTP_PORT", "8080")
	jwtSecret := []byte(getEnvString("JWT_SECRET", "supersecretkey"))

	// Load database configuration
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg := &Config{
		HTTPPort:  port,
		JWTSecret: jwtSecret,
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
	}

	return cfg, nil
}
