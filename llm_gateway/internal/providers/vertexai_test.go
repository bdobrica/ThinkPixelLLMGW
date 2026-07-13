package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVertexAIChatForwardsOpenAIContractAndUsage(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer vertex-token" {
			t.Fatalf("unexpected authorization %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":11,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":2}}}`)
	}))
	defer server.Close()

	provider, err := NewVertexAIProvider(ProviderConfig{
		ID: "vertex-id", Name: "Vertex", Credentials: map[string]string{"access_token": "vertex-token"},
		Config: map[string]any{"project_id": "project", "location": "europe-west4", "base_url": server.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hi"}}}
	response, err := provider.Chat(context.Background(), ChatRequest{Model: "google/gemini-2.5-flash", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.InputTokens != 11 || response.OutputTokens != 7 || response.CachedTokens != 3 || response.ReasoningTokens != 2 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if received["model"] != "google/gemini-2.5-flash" {
		t.Fatalf("model was not translated: %#v", received)
	}
	if _, mutated := payload["model"]; mutated {
		t.Fatal("Chat mutated the caller payload")
	}
}

func TestVertexAIChatStreamsAndMapsHTTPError(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
		}))
		defer server.Close()
		provider := newTestVertexProvider(t, server.URL, nil)
		response, err := provider.Chat(context.Background(), ChatRequest{Model: "gemini", Stream: true, Payload: map[string]any{"stream": true}})
		if err != nil {
			t.Fatal(err)
		}
		defer response.Stream.Close()
		body, _ := io.ReadAll(response.Stream)
		if !strings.Contains(string(body), "[DONE]") {
			t.Fatalf("stream was not passed through: %s", body)
		}
	})

	t.Run("error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":{"message":"quota"}}`)
		}))
		defer server.Close()
		provider := newTestVertexProvider(t, server.URL, nil)
		response, err := provider.Chat(context.Background(), ChatRequest{Model: "gemini", Payload: map[string]any{}})
		if err != nil || response.StatusCode != http.StatusTooManyRequests || !strings.Contains(string(response.Body), "quota") {
			t.Fatalf("unexpected error response: response=%+v err=%v", response, err)
		}
	})
}

func TestVertexAIValidationAndCancellation(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer vertex-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		provider := newTestVertexProvider(t, server.URL, map[string]any{"validation_url": server.URL})
		if err := provider.ValidateCredentials(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()
		provider := newTestVertexProvider(t, server.URL, map[string]any{"request_timeout": "5s"})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := provider.Chat(ctx, ChatRequest{Model: "gemini", Payload: map[string]any{}})
		if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("expected cancellation error, got %v", err)
		}
	})
}

func TestVertexAIConfigurationValidation(t *testing.T) {
	if _, err := NewVertexAIProvider(ProviderConfig{Credentials: map[string]string{"access_token": "token"}, Config: map[string]any{}}); err == nil {
		t.Fatal("expected missing project_id error")
	}
	if _, err := NewVertexAIProvider(ProviderConfig{Credentials: map[string]string{"service_account_json": "not-json"}, Config: map[string]any{"project_id": "p"}}); err == nil {
		t.Fatal("expected malformed service account error")
	}
}

func newTestVertexProvider(t *testing.T, baseURL string, extra map[string]any) *VertexAIProvider {
	t.Helper()
	config := map[string]any{"project_id": "project", "base_url": baseURL}
	for key, value := range extra {
		config[key] = value
	}
	provider, err := NewVertexAIProvider(ProviderConfig{ID: "vertex", Name: "Vertex", Credentials: map[string]string{"access_token": "vertex-token"}, Config: config})
	if err != nil {
		t.Fatal(err)
	}
	return provider.(*VertexAIProvider)
}
