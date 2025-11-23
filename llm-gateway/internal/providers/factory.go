package providers

import (
	"fmt"
	"sync"
)

// ProviderFactory is the concrete implementation of the Factory interface
type ProviderFactory struct {
	mu        sync.RWMutex
	creators  map[string]ProviderCreator
}

// ProviderCreator is a function that creates a provider instance
type ProviderCreator func(config ProviderConfig) (Provider, error)

// NewProviderFactory creates a new provider factory with default providers registered
func NewProviderFactory() *ProviderFactory {
	f := &ProviderFactory{
		creators: make(map[string]ProviderCreator),
	}
	
	// Register built-in providers
	f.Register("openai", NewOpenAIProvider)
	f.Register("vertexai", NewVertexAIProvider)
	f.Register("bedrock", NewBedrockProvider)
	
	return f
}

// Register registers a provider creator for a specific type
func (f *ProviderFactory) Register(providerType string, creator ProviderCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creators[providerType] = creator
}

// CreateProvider creates a new provider instance based on the configuration
func (f *ProviderFactory) CreateProvider(config ProviderConfig) (Provider, error) {
	f.mu.RLock()
	creator, exists := f.creators[config.Type]
	f.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}
	
	provider, err := creator(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider %s (%s): %w", config.Name, config.Type, err)
	}
	
	return provider, nil
}

// SupportedTypes returns the list of supported provider types
func (f *ProviderFactory) SupportedTypes() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	types := make([]string, 0, len(f.creators))
	for t := range f.creators {
		types = append(types, t)
	}
	return types
}
