package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/metrics"
	"llm_gateway/internal/providers"
)

// BenchmarkNonStreamingResponse measures the in-process gateway response and
// accounting path with provider, database, Redis, and network time excluded.
// It is a regression baseline, not a production capacity claim.
func BenchmarkNonStreamingResponse(b *testing.B) {
	deps := &Dependencies{Metrics: metrics.NewNoopMetrics()}
	apiKey := &auth.APIKeyRecord{
		ID:   uuid.NewString(),
		Name: "benchmark-key",
		Tags: map[string]string{"profile": "release-qualification"},
	}
	provider := &stubProvider{}
	response := &providers.ChatResponse{
		StatusCode:      http.StatusOK,
		Body:            []byte(`{"id":"benchmark","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":8,"completion_tokens":1,"total_tokens":9}}`),
		InputTokens:     8,
		OutputTokens:    1,
		ProviderLatency: time.Millisecond,
	}
	payload := map[string]any{
		"model":    "benchmark-model",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			deps.handleNonStreamingResponse(
				recorder,
				response,
				apiKey,
				uuid.NewString(),
				"benchmark-model",
				"benchmark-model",
				provider,
				payload,
				time.Now(),
				time.Millisecond,
				nil,
			)
			if recorder.Code != http.StatusOK {
				b.Fatalf("unexpected response status: %d", recorder.Code)
			}
		}
	})
}
