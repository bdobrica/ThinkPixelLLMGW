package providers

import (
	"context"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/models"
)

// InMemoryRegistry is a simple placeholder registry.
// For now, it has a single default provider and does not actually map aliases.
type InMemoryRegistry struct {
	defaultProvider Provider
}

// NewInMemoryRegistry constructs a registry with one OpenAI provider.
func NewInMemoryRegistry() *InMemoryRegistry {
	// In a real impl, you'd load providers & aliases from DB.
	return &InMemoryRegistry{
		defaultProvider: NewOpenAIProvider("openai-default"),
	}
}

// ResolveModel currently always returns the default provider and passes through
// the provided model name unchanged.
func (r *InMemoryRegistry) ResolveModel(ctx context.Context, model string) (Provider, string, error) {
	if r.defaultProvider == nil {
		// In practice this should never happen once wired properly.
		return nil, "", ErrProviderNotFound
	}
	return r.defaultProvider, model, nil
}

// Optional typed error(s) for callers to inspect.
var ErrProviderNotFound = models.ErrNotFound // or define your own if you like
