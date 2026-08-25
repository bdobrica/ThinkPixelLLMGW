package httpapi

import (
	"net/http"
	"testing"

	"llm_gateway/internal/config"
	"llm_gateway/internal/metrics"
)

func TestResponsesRoutesAreBehindFeatureFlag(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		want    string
	}{
		{"disabled", false, ""},
		{"enabled", true, "/v1/responses"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			deps := &Dependencies{Metrics: metrics.NewPrometheusMetrics()}
			registerRoutes(mux, deps, &config.Config{Responses: config.ResponsesConfig{Enabled: test.enabled}})
			request, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, pattern := mux.Handler(request)
			if pattern != test.want {
				t.Fatalf("route pattern = %q, want %q", pattern, test.want)
			}
		})
	}
}
