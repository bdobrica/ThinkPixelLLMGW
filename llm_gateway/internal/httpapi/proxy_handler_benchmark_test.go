package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/metrics"
	"llm_gateway/internal/providers"
)

func TestNonStreamingResponseSoak(t *testing.T) {
	rawDuration := os.Getenv("RESPONSE_SOAK_DURATION")
	if rawDuration == "" {
		t.Skip("set RESPONSE_SOAK_DURATION to run the response/accounting soak")
	}
	duration, err := time.ParseDuration(rawDuration)
	if err != nil || duration <= 0 {
		t.Fatalf("invalid RESPONSE_SOAK_DURATION %q", rawDuration)
	}

	deps, apiKey, provider, response, payload := benchmarkResponseFixture()
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	var operations atomic.Uint64
	var workers sync.WaitGroup
	errCh := make(chan int, 1)
	for range runtime.GOMAXPROCS(0) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				recorder := httptest.NewRecorder()
				deps.handleNonStreamingResponse(recorder, response, apiKey, uuid.NewString(), "benchmark-model", "benchmark-model", provider, payload, time.Now(), time.Millisecond, nil)
				if recorder.Code != http.StatusOK {
					select {
					case errCh <- recorder.Code:
					default:
					}
					cancel()
					return
				}
				operations.Add(1)
			}
		}()
	}
	workers.Wait()
	select {
	case status := <-errCh:
		t.Fatalf("unexpected response status: %d", status)
	default:
	}
	t.Logf("completed %d operations over %s with %d workers", operations.Load(), duration, runtime.GOMAXPROCS(0))
}

// BenchmarkNonStreamingResponse measures the in-process gateway response and
// accounting path with provider, database, Redis, and network time excluded.
// It is a regression baseline, not a production capacity claim.
func BenchmarkNonStreamingResponse(b *testing.B) {
	deps, apiKey, provider, response, payload := benchmarkResponseFixture()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			recorder := httptest.NewRecorder()
			deps.handleNonStreamingResponse(recorder, response, apiKey, uuid.NewString(), "benchmark-model", "benchmark-model", provider, payload, time.Now(), time.Millisecond, nil)
			if recorder.Code != http.StatusOK {
				b.Fatalf("unexpected response status: %d", recorder.Code)
			}
		}
	})
}

func benchmarkResponseFixture() (*Dependencies, *auth.APIKeyRecord, providers.Provider, *providers.ChatResponse, map[string]any) {
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

	return deps, apiKey, provider, response, payload
}
