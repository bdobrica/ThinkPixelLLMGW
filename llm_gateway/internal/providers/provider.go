package providers

import (
	"context"
	"io"
	"time"
)

// ChatRequest represents a normalized internal request to a provider.
type ChatRequest struct {
	Model   string         // provider-specific model name
	Payload map[string]any // OpenAI-style payload as generic JSON
	Stream  bool           // whether to stream the response
}

// ChatResponse is a normalized provider response.
type ChatResponse struct {
	StatusCode      int
	Body            []byte
	Stream          io.ReadCloser // non-nil if streaming
	ProviderLatency time.Duration
	CostUSD         float64
	// Usage information extracted from response
	InputTokens     int
	OutputTokens    int
	CachedTokens    int
	ReasoningTokens int
}

// StreamEvent represents a single event in a streaming response
type StreamEvent struct {
	Data  []byte
	Error error
	Done  bool
}

// Provider is implemented by each concrete LLM provider (OpenAI, Vertex, Bedrock, ...).
type Provider interface {
	// ID returns the unique identifier for this provider instance
	ID() string

	// Name returns the display name of this provider
	Name() string

	// Type returns the provider type (openai, vertexai, bedrock, etc.)
	Type() string

	// Chat sends a chat completion request to the provider
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ValidateCredentials checks if the provider credentials are valid
	ValidateCredentials(ctx context.Context) error

	// Close performs cleanup when the provider is no longer needed
	Close() error
}

// Authenticator handles authentication for a provider.
// Different providers implement different authentication mechanisms:
// - Simple: API key in header (OpenAI)
// - SDK-based: AWS SDK with IAM (Bedrock), Google Cloud SDK (VertexAI)
type Authenticator interface {
	// Authenticate prepares authentication for a request
	// For simple auth, this might set headers
	// For SDK-based auth, this might return an authenticated client
	Authenticate(ctx context.Context) (AuthContext, error)
}

// AuthContext holds authentication information for a request
type AuthContext interface {
	// ApplyToRequest applies authentication to an HTTP request or SDK client
	ApplyToRequest(ctx context.Context, req any) error
}

// ProviderConfig holds configuration for creating a provider instance
type ProviderConfig struct {
	ID          string
	Name        string
	Type        string
	Credentials map[string]string // decrypted credentials
	Config      map[string]any    // additional configuration
}

// Factory creates provider instances based on type and configuration
type Factory interface {
	// CreateProvider creates a new provider instance
	CreateProvider(config ProviderConfig) (Provider, error)

	// SupportedTypes returns the list of supported provider types
	SupportedTypes() []string
}

// Registry resolves model names/aliases to providers and manages provider lifecycle
type Registry interface {
	// ResolveModel resolves a model or alias name to a provider and model name
	ResolveModel(ctx context.Context, modelNameOrAlias string) (Provider, string, error)

	// GetProvider retrieves a provider by ID
	GetProvider(ctx context.Context, providerID string) (Provider, error)

	// ListProviders returns all active providers
	ListProviders(ctx context.Context) ([]Provider, error)

	// Reload reloads all providers from the database
	Reload(ctx context.Context) error

	// Close closes all providers and cleans up resources
	Close() error
}
