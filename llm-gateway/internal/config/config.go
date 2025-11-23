package config

import "os"

// Config holds configuration for the gateway.
type Config struct {
	HTTPPort string
	// TODO: add DB DSN, Redis address, encryption keys, etc.
}

// Load reads configuration from environment variables (and, later, other sources).
func Load() (*Config, error) {
	port := os.Getenv("GATEWAY_HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		HTTPPort: port,
	}, nil
}
