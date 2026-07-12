package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadHTTPServerTimeouts(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("GATEWAY_HTTP_PORT", "9090")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "2s")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "4s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "5s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "6s")
	t.Setenv("HTTP_READINESS_TIMEOUT", "7s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPPort != "9090" || cfg.HTTPServer.ReadHeaderTimeout != 2*time.Second || cfg.HTTPServer.ReadTimeout != 3*time.Second || cfg.HTTPServer.WriteTimeout != 4*time.Second || cfg.HTTPServer.IdleTimeout != 5*time.Second || cfg.HTTPServer.ShutdownTimeout != 6*time.Second || cfg.HTTPServer.ReadinessTimeout != 7*time.Second {
		t.Fatalf("unexpected HTTP config: %#v", cfg.HTTPServer)
	}
}

func TestLoadRejectsInvalidReadinessTimeout(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("HTTP_READINESS_TIMEOUT", "invalid")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "HTTP_READINESS_TIMEOUT") {
		t.Fatalf("expected readiness timeout validation error, got %v", err)
	}
}

func TestLoadRejectsInvalidHTTPTimeout(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("HTTP_WRITE_TIMEOUT", "0")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "HTTP_WRITE_TIMEOUT") {
		t.Fatalf("expected HTTP_WRITE_TIMEOUT validation error, got %v", err)
	}
}
