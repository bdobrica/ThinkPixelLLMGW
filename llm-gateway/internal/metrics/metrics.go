package metrics

import "net/http"

// Metrics exposes gateway metrics (e.g. Prometheus handler).
type Metrics interface {
	HTTPHandler() http.Handler
}

// NoopMetrics is a placeholder metrics implementation.
type NoopMetrics struct{}

func NewNoopMetrics() *NoopMetrics {
	return &NoopMetrics{}
}

func (m *NoopMetrics) HTTPHandler() http.Handler {
	// For now, just respond 204.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}
