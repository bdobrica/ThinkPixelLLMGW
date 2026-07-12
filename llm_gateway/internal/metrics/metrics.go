package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics exposes gateway metrics (e.g. Prometheus handler).
type Metrics interface {
	HTTPHandler() http.Handler
	RecordRequest(apiKeyID, apiKeyName string, tags map[string]string, inputTokens, cachedTokens, outputTokens int, costUSD float64, latency time.Duration)
	RecordStreamUsageMissing(provider, model, reason string)
}

// PrometheusMetrics implements Metrics using Prometheus
type PrometheusMetrics struct {
	requestsTotal      *prometheus.CounterVec
	inputTokensTotal   *prometheus.CounterVec
	cachedTokensTotal  *prometheus.CounterVec
	outputTokensTotal  *prometheus.CounterVec
	costTotal          *prometheus.CounterVec
	requestLatency     *prometheus.HistogramVec
	streamUsageMissing *prometheus.CounterVec
	registry           *prometheus.Registry
}

// NewPrometheusMetrics creates a new Prometheus metrics collector
func NewPrometheusMetrics() *PrometheusMetrics {
	registry := prometheus.NewRegistry()

	// Define label names including API key and dynamic tag labels
	// We'll use a fixed set of common tag labels for now
	labelNames := []string{"api_key_id", "api_key_name"}

	m := &PrometheusMetrics{
		registry: registry,
		requestsTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_gateway_requests_total",
				Help: "Total number of LLM API requests",
			},
			labelNames,
		),
		inputTokensTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_gateway_input_tokens_total",
				Help: "Total number of input tokens processed",
			},
			labelNames,
		),
		cachedTokensTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_gateway_cached_tokens_total",
				Help: "Total number of cached input tokens",
			},
			labelNames,
		),
		outputTokensTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_gateway_output_tokens_total",
				Help: "Total number of output tokens generated",
			},
			labelNames,
		),
		costTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_gateway_cost_usd_total",
				Help: "Total cost tracked in USD",
			},
			labelNames,
		),
		requestLatency: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "llm_gateway_request_duration_seconds",
				Help:    "Request latency distribution in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0, 60.0},
			},
			labelNames,
		),
		streamUsageMissing: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_gateway_stream_usage_missing_total",
				Help: "Streaming responses without provider-reported terminal usage",
			},
			[]string{"provider", "model", "reason"},
		),
	}

	return m
}

func (m *PrometheusMetrics) RecordStreamUsageMissing(provider, model, reason string) {
	m.streamUsageMissing.WithLabelValues(provider, model, reason).Inc()
}

// RecordRequest records metrics for a completed request
func (m *PrometheusMetrics) RecordRequest(apiKeyID, apiKeyName string, tags map[string]string, inputTokens, cachedTokens, outputTokens int, costUSD float64, latency time.Duration) {
	// Create label values - start with API key info
	labels := prometheus.Labels{
		"api_key_id":   apiKeyID,
		"api_key_name": apiKeyName,
	}

	// Note: We don't dynamically add tag labels here because Prometheus requires
	// consistent label names. Tags should be queried from the API key metadata
	// or stored in a separate system for correlation.
	// For now, we keep the metric labels minimal and consistent.

	m.requestsTotal.With(labels).Inc()
	m.inputTokensTotal.With(labels).Add(float64(inputTokens))
	m.cachedTokensTotal.With(labels).Add(float64(cachedTokens))
	m.outputTokensTotal.With(labels).Add(float64(outputTokens))
	m.costTotal.With(labels).Add(costUSD)
	m.requestLatency.With(labels).Observe(latency.Seconds())
}

// HTTPHandler returns the Prometheus HTTP handler
func (m *PrometheusMetrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
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

func (m *NoopMetrics) RecordRequest(apiKeyID, apiKeyName string, tags map[string]string, inputTokens, cachedTokens, outputTokens int, costUSD float64, latency time.Duration) {
	// No-op
}

func (m *NoopMetrics) RecordStreamUsageMissing(provider, model, reason string) {}
