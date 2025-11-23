package httpapi

import (
	"net/http"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/auth"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/billing"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/config"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/logging"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/metrics"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/providers"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/ratelimit"
)

// Dependencies aggregates all services the HTTP layer needs.
type Dependencies struct {
	APIKeys   auth.APIKeyStore
	Providers providers.Registry
	RateLimit ratelimit.Limiter
	Billing   billing.Service
	Logger    logging.Sink
	Metrics   metrics.Metrics
	JWT       auth.JWTManager
}

// NewRouter wires basic dependencies and routes.
// Later you can replace the Noop implementations with real ones.
func NewRouter(cfg *config.Config) (*http.ServeMux, error) {
	deps := &Dependencies{
		APIKeys:   auth.NewInMemoryAPIKeyStore(),
		Providers: providers.NewInMemoryRegistry(),
		RateLimit: ratelimit.NewNoopLimiter(),
		Billing:   billing.NewNoopService(),
		Logger:    logging.NewNoopSink(),
		Metrics:   metrics.NewNoopMetrics(),
		JWT:       auth.NewNoopJWTManager(),
	}

	mux := http.NewServeMux()
	registerRoutes(mux, deps)
	return mux, nil
}

func registerRoutes(mux *http.ServeMux, deps *Dependencies) {
	// Public OpenAI-compatible proxy endpoint(s)
	mux.HandleFunc("/v1/chat/completions", deps.handleChat)

	// Admin API (JWT-protected)
	mux.HandleFunc("/admin/keys", deps.withJWTAuth(deps.handleAdminKeys))
	mux.HandleFunc("/admin/providers", deps.withJWTAuth(deps.handleAdminProviders))

	// Metrics endpoint
	mux.Handle("/metrics", deps.Metrics.HTTPHandler())
}
