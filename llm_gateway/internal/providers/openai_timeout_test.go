package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIProviderCreateResponseUsesNativeEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected request: %s %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"gpt-test"`) {
			t.Fatalf("unexpected body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_upstream","usage":{"input_tokens":2,"output_tokens":3,"input_tokens_details":{"cached_tokens":1},"output_tokens_details":{"reasoning_tokens":1}}}`))
	}))
	defer server.Close()
	providerValue, err := NewOpenAIProvider(ProviderConfig{ID: "id", Name: "test", Credentials: map[string]string{"api_key": "test-key"}, Config: map[string]any{"base_url": server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	provider := providerValue.(*OpenAIProvider)
	response, err := provider.CreateResponse(context.Background(), ResponsesRequest{Payload: []byte(`{"model":"gpt-test","input":"hi"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if response.InputTokens != 2 || response.OutputTokens != 3 || response.CachedTokens != 1 || response.ReasoningTokens != 1 {
		t.Fatalf("unexpected usage: %#v", response)
	}
}

func TestOpenAIProviderTimesOutStalledUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID: "openai-test", Name: "OpenAI Test", Type: "openai",
		Credentials: map[string]string{"api_key": "test-key"},
		Config:      map[string]any{"base_url": upstream.URL, "request_timeout": "50ms"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Chat(context.Background(), ChatRequest{Model: "test", Payload: map[string]any{"model": "test"}})
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("expected stalled upstream timeout, got %v", err)
	}
}

func TestNewOpenAIProvider_UsesConfiguredRequestTimeout(t *testing.T) {
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID:   "openai-test",
		Name: "OpenAI Test",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "test-key",
		},
		Config: map[string]any{
			"request_timeout": "15s",
		},
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed: %v", err)
	}

	op, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider type")
	}

	if op.client.Timeout != 15*time.Second {
		t.Fatalf("expected timeout 15s, got %v", op.client.Timeout)
	}
}

func TestNewOpenAIProvider_UsesDefaultTimeoutWhenUnset(t *testing.T) {
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID:   "openai-test",
		Name: "OpenAI Test",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "test-key",
		},
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed: %v", err)
	}

	op, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider type")
	}

	if op.client.Timeout != openAITimeout {
		t.Fatalf("expected default timeout %v, got %v", openAITimeout, op.client.Timeout)
	}
}

func TestNewOpenAIProvider_IgnoresInvalidRequestTimeout(t *testing.T) {
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID:   "openai-test",
		Name: "OpenAI Test",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "test-key",
		},
		Config: map[string]any{
			"request_timeout": "not-a-duration",
		},
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed: %v", err)
	}

	op, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider type")
	}

	if op.client.Timeout != openAITimeout {
		t.Fatalf("expected default timeout %v, got %v", openAITimeout, op.client.Timeout)
	}
}
