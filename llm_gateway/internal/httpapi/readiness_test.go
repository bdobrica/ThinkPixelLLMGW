package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"llm_gateway/internal/metrics"
)

func TestReadinessHandler(t *testing.T) {
	tests := []struct {
		name     string
		checks   []readinessCheck
		wantCode int
		wantBody string
	}{
		{"healthy", []readinessCheck{{"db", func(context.Context) error { return nil }}, {"redis", func(context.Context) error { return nil }}}, http.StatusOK, `{"status":"ready"}` + "\n"},
		{"database down", []readinessCheck{{"db", func(context.Context) error { return errors.New("down") }}, {"redis", func(context.Context) error { return nil }}}, http.StatusServiceUnavailable, `{"status":"unavailable"}` + "\n"},
		{"redis down", []readinessCheck{{"db", func(context.Context) error { return nil }}, {"redis", func(context.Context) error { return errors.New("down") }}}, http.StatusServiceUnavailable, `{"status":"unavailable"}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			readinessHandlerForChecks(tt.checks, time.Second, metrics.NewNoopMetrics()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if recorder.Code != tt.wantCode || recorder.Body.String() != tt.wantBody {
				t.Fatalf("response = %d %q, want %d %q", recorder.Code, recorder.Body.String(), tt.wantCode, tt.wantBody)
			}
		})
	}
}

func TestReadinessHandlerTimesOutWithoutExposingError(t *testing.T) {
	checks := []readinessCheck{{"secret dependency", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}}
	recorder := httptest.NewRecorder()
	readinessHandlerForChecks(checks, 5*time.Millisecond, metrics.NewNoopMetrics()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.String() != `{"status":"unavailable"}`+"\n" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestLivenessDoesNotRunReadinessChecks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("OK")) })
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "OK" {
		t.Fatalf("liveness = %d %q", recorder.Code, recorder.Body.String())
	}
}
