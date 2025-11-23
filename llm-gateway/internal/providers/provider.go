package providers

import (
	"context"
	"time"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/models"
)

// ChatRequest represents a normalized internal request to a provider.
type ChatRequest struct {
	Model   string                 // provider-specific model name
	Payload map[string]any         // OpenAI-style payload as generic JSON
	// TODO: add headers, timeouts, etc. if needed
}

// ChatResponse is a normalized provider response.
type ChatResponse struct {
	StatusCode      int
	Body            []byte
	ProviderLatency time.Duration
	CostUSD         float64
}

// Provider is implemented by each concrete LLM provider (OpenAI, Vertex, Bedrock, ...).
type Provider interface {
	ID() string
	Type() models.ProviderType
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Registry resolves a model/alias name into a provider + provider model name.
type Registry interface {
	ResolveModel(ctx context.Context, model string) (Provider, string, error)
}
