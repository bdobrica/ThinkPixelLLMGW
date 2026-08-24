package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadHTTPServerTimeouts(t *testing.T) {
	setRequiredConfig(t)
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
	setRequiredConfig(t)
	t.Setenv("HTTP_READINESS_TIMEOUT", "invalid")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "HTTP_READINESS_TIMEOUT") {
		t.Fatalf("expected readiness timeout validation error, got %v", err)
	}
}

func TestLoadRejectsInvalidHTTPTimeout(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("HTTP_WRITE_TIMEOUT", "0")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "HTTP_WRITE_TIMEOUT") {
		t.Fatalf("expected HTTP_WRITE_TIMEOUT validation error, got %v", err)
	}
}

func TestLoadRejectsWeakRequiredSecrets(t *testing.T) {
	for _, key := range []string{"JWT_SECRET", "METRICS_AUTH_TOKEN", "ENCRYPTION_KEY"} {
		t.Run(key, func(t *testing.T) {
			setRequiredConfig(t)
			t.Setenv(key, "weak")
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("expected %s validation error, got %v", key, err)
			}
		})
	}
}

func TestLoadValidatesEnabledS3Sink(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("LOGGING_SINK_ENABLED", "true")
	t.Setenv("LOGGING_SINK_S3_BUCKET", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "LOGGING_SINK_S3_BUCKET") {
		t.Fatalf("expected bucket validation error, got %v", err)
	}
}

func TestLoadAuditPrivacyPolicy(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("AUDIT_BODY_MODE", "redacted")
	t.Setenv("AUDIT_MAX_BODY_BYTES", "2048")
	t.Setenv("AUDIT_SAMPLE_RATE", "0.25")
	t.Setenv("AUDIT_SENSITIVE_FIELDS", "ssn,customer_secret")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RequestLogger.BodyMode != "redacted" || cfg.RequestLogger.MaxBodyBytes != 2048 || cfg.RequestLogger.SampleRate != 0.25 {
		t.Fatalf("unexpected policy: %#v", cfg.RequestLogger)
	}
}

func TestLoadRejectsInvalidAuditPrivacyPolicy(t *testing.T) {
	for key, value := range map[string]string{"AUDIT_BODY_MODE": "plaintext", "AUDIT_MAX_BODY_BYTES": "0", "AUDIT_SAMPLE_RATE": "1.1"} {
		t.Run(key, func(t *testing.T) {
			setRequiredConfig(t)
			t.Setenv(key, value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("expected %s error, got %v", key, err)
			}
		})
	}
}

func TestLoadResponsesStatePolicy(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("RESPONSES_RETENTION", "48h")
	t.Setenv("RESPONSES_TRANSIENT_RETENTION", "30m")
	t.Setenv("RESPONSES_ORPHANED_AFTER", "5m")
	t.Setenv("RESPONSES_MAX_CHAIN_DEPTH", "12")
	t.Setenv("RESPONSES_MAX_CHAIN_ITEMS", "100")
	t.Setenv("RESPONSES_MAX_CHAIN_BYTES", "2048")
	t.Setenv("RESPONSES_MAX_CHAIN_TOKENS", "512")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Responses.Retention != 48*time.Hour || cfg.Responses.TransientRetention != 30*time.Minute ||
		cfg.Responses.OrphanedAfter != 5*time.Minute || cfg.Responses.MaxChainDepth != 12 ||
		cfg.Responses.MaxChainItems != 100 || cfg.Responses.MaxChainBytes != 2048 || cfg.Responses.MaxChainTokens != 512 {
		t.Fatalf("unexpected Responses config: %#v", cfg.Responses)
	}
}

func TestLoadRejectsInvalidResponsesStatePolicy(t *testing.T) {
	for key, value := range map[string]string{
		"RESPONSES_RETENTION": "0", "RESPONSES_TRANSIENT_RETENTION": "invalid",
		"RESPONSES_ORPHANED_AFTER": "-1s", "RESPONSES_MAX_CHAIN_DEPTH": "0",
		"RESPONSES_MAX_CHAIN_ITEMS": "-1", "RESPONSES_MAX_CHAIN_BYTES": "0", "RESPONSES_MAX_CHAIN_TOKENS": "0",
	} {
		t.Run(key, func(t *testing.T) {
			setRequiredConfig(t)
			t.Setenv(key, value)
			_, err := Load()
			if err == nil {
				t.Fatalf("expected %s validation error", key)
			}
		})
	}
}

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("METRICS_AUTH_TOKEN", "abcdef0123456789abcdef0123456789")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("LOGGING_SINK_ENABLED", "false")
}
