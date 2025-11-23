package providers

import "context"

// InMemoryRegistry is a simple placeholder registry.
// Later you'll probably back this with DB + caches.
type InMemoryRegistry struct {
	// TODO: add maps from alias → provider/model, providerID → provider, etc.
}

func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{}
}

// ResolveModel currently returns no provider.
// Wire it to your model alias table and provider set.
func (r *InMemoryRegistry) ResolveModel(ctx context.Context, model string) (Provider, string, error) {
	// TODO: look up model alias / provider here
	return nil, "", nil
}
