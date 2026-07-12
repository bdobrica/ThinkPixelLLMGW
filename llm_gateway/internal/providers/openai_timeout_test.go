package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
