package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds configuration for the gateway.
type Config struct {
	HTTPPort string
	Database DatabaseConfig
	Cache    CacheConfig
	// TODO: add Redis address, encryption keys, etc.
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

// Load reads configuration from environment variables (and, later, other sources).
func Load() (*Config, error) {
	port := os.Getenv("GATEWAY_HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	// Load database configuration
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cfg := &Config{
		HTTPPort: port,
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
	}

	return cfg, nil
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
