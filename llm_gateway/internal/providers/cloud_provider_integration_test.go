//go:build integration

package providers

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestOpenAILiveChat(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("set OPENAI_API_KEY to run the live OpenAI smoke test")
	}
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID: "live-openai", Name: "Live OpenAI",
		Credentials: map[string]string{"api_key": apiKey},
		Config:      map[string]any{"request_timeout": "60s"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := provider.Chat(ctx, ChatRequest{Model: envOrDefault("OPENAI_TEST_MODEL", "gpt-4o-mini"), Payload: liveSmokePayload()})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("OpenAI status=%d body=%s", response.StatusCode, response.Body)
	}
}

func TestVertexAILiveChat(t *testing.T) {
	projectID := os.Getenv("VERTEX_TEST_PROJECT_ID")
	model := os.Getenv("VERTEX_TEST_MODEL")
	if projectID == "" || model == "" {
		t.Skip("set VERTEX_TEST_PROJECT_ID and VERTEX_TEST_MODEL to run the live Vertex AI smoke test")
	}
	provider, err := NewVertexAIProvider(ProviderConfig{
		ID: "live-vertex", Name: "Live Vertex AI",
		Config: map[string]any{"project_id": projectID, "location": envOrDefault("VERTEX_TEST_LOCATION", "us-central1"), "request_timeout": "60s"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := provider.Chat(ctx, ChatRequest{Model: model, Payload: liveSmokePayload()})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("Vertex AI status=%d body=%s", response.StatusCode, response.Body)
	}
}

func TestBedrockLiveChat(t *testing.T) {
	model := os.Getenv("BEDROCK_TEST_MODEL")
	if model == "" {
		t.Skip("set BEDROCK_TEST_MODEL and AWS SDK credentials to run the live Bedrock smoke test")
	}
	provider, err := NewBedrockProvider(ProviderConfig{
		ID: "live-bedrock", Name: "Live Bedrock",
		Config: map[string]any{"region": envOrDefault("BEDROCK_TEST_REGION", "us-east-1"), "request_timeout": "60s"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	response, err := provider.Chat(ctx, ChatRequest{Model: model, Payload: liveSmokePayload()})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("Bedrock status=%d body=%s", response.StatusCode, response.Body)
	}
}

func liveSmokePayload() map[string]any {
	return map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Reply with the single word OK."}}, "max_tokens": 8}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
