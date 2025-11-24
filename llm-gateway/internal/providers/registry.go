package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gateway/internal/storage"
)

// ProviderRegistry manages all provider instances and resolves models to providers
type ProviderRegistry struct {
	factory    Factory
	db         *storage.DB
	encryption *storage.Encryption

	mu              sync.RWMutex
	providers       map[string]Provider // provider ID -> Provider instance
	modelToProvider map[string]string   // model name -> provider ID
	aliasToProvider map[string]string   // alias -> provider ID
	aliasToModel    map[string]string   // alias -> actual model name

	reloadInterval time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// RegistryConfig holds configuration for the provider registry
type RegistryConfig struct {
	Factory        Factory
	DB             *storage.DB
	Encryption     *storage.Encryption
	ReloadInterval time.Duration // how often to reload providers from DB (0 = no auto-reload)
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry(config RegistryConfig) (*ProviderRegistry, error) {
	if config.Factory == nil {
		config.Factory = NewProviderFactory()
	}

	if config.ReloadInterval == 0 {
		config.ReloadInterval = 5 * time.Minute // default reload interval
	}

	r := &ProviderRegistry{
		factory:         config.Factory,
		db:              config.DB,
		encryption:      config.Encryption,
		providers:       make(map[string]Provider),
		modelToProvider: make(map[string]string),
		aliasToProvider: make(map[string]string),
		aliasToModel:    make(map[string]string),
		reloadInterval:  config.ReloadInterval,
		stopCh:          make(chan struct{}),
	}

	// Initial load
	if err := r.Reload(context.Background()); err != nil {
		return nil, fmt.Errorf("failed initial provider load: %w", err)
	}

	// Start background reload if interval > 0
	if config.ReloadInterval > 0 {
		r.wg.Add(1)
		go r.reloadLoop()
	}

	return r, nil
}

// ResolveModel resolves a model name or alias to a provider and actual model name
func (r *ProviderRegistry) ResolveModel(ctx context.Context, modelNameOrAlias string) (Provider, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// First check if it's an alias
	if providerID, exists := r.aliasToProvider[modelNameOrAlias]; exists {
		provider, ok := r.providers[providerID]
		if !ok {
			return nil, "", fmt.Errorf("provider %s not found for alias %s", providerID, modelNameOrAlias)
		}

		modelName := r.aliasToModel[modelNameOrAlias]
		return provider, modelName, nil
	}

	// Check if it's a direct model name
	if providerID, exists := r.modelToProvider[modelNameOrAlias]; exists {
		provider, ok := r.providers[providerID]
		if !ok {
			return nil, "", fmt.Errorf("provider %s not found for model %s", providerID, modelNameOrAlias)
		}

		return provider, modelNameOrAlias, nil
	}

	return nil, "", fmt.Errorf("model or alias not found: %s", modelNameOrAlias)
}

// GetProvider retrieves a provider by ID
func (r *ProviderRegistry) GetProvider(ctx context.Context, providerID string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return provider, nil
}

// ListProviders returns all active providers
func (r *ProviderRegistry) ListProviders(ctx context.Context) ([]Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}

	return providers, nil
}

// Reload reloads all providers from the database
func (r *ProviderRegistry) Reload(ctx context.Context) error {
	// Load providers from database
	providerRepo := storage.NewProviderRepository(r.db)
	dbProviders, err := providerRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to load providers from database: %w", err)
	}

	// Load model aliases
	aliasRepo := storage.NewModelAliasRepository(r.db)
	aliases, err := aliasRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to load aliases from database: %w", err)
	}

	// Load models to map them to providers
	modelRepo := storage.NewModelRepository(r.db)
	models, err := modelRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to load models from database: %w", err)
	}

	// Build new provider instances
	newProviders := make(map[string]Provider)
	newModelToProvider := make(map[string]string)
	newAliasToProvider := make(map[string]string)
	newAliasToModel := make(map[string]string)

	for _, dbProvider := range dbProviders {
		if !dbProvider.Enabled {
			continue
		}

		// Decrypt credentials
		credentials := make(map[string]string)
		if len(dbProvider.EncryptedCredentials) > 0 && r.encryption != nil {
			decrypted, err := r.encryption.Decrypt(dbProvider.EncryptedCredentials)
			if err != nil {
				return fmt.Errorf("failed to decrypt credentials for provider %s: %w", dbProvider.Name, err)
			}

			if err := json.Unmarshal(decrypted, &credentials); err != nil {
				return fmt.Errorf("failed to unmarshal credentials for provider %s: %w", dbProvider.Name, err)
			}
		}

		// Parse config
		config := make(map[string]any)
		if len(dbProvider.Config) > 0 {
			if err := json.Unmarshal(dbProvider.Config, &config); err != nil {
				return fmt.Errorf("failed to unmarshal config for provider %s: %w", dbProvider.Name, err)
			}
		}

		// Create provider instance
		providerConfig := ProviderConfig{
			ID:          dbProvider.ID.String(),
			Name:        dbProvider.DisplayName,
			Type:        dbProvider.ProviderType,
			Credentials: credentials,
			Config:      config,
		}

		provider, err := r.factory.CreateProvider(providerConfig)
		if err != nil {
			return fmt.Errorf("failed to create provider %s: %w", dbProvider.Name, err)
		}

		newProviders[dbProvider.ID.String()] = provider
	}

	// Map models to providers
	for _, model := range models {
		// Find which provider(s) support this model by matching litellm_provider
		for _, dbProvider := range dbProviders {
			if !dbProvider.Enabled {
				continue
			}

			// Simple heuristic: match provider type to litellm_provider
			// In production, you might have a more sophisticated mapping
			if matchesLiteLLMProvider(dbProvider.ProviderType, model.LiteLLMProvider) {
				newModelToProvider[model.ModelName] = dbProvider.ID.String()
				break // Use first matching provider
			}
		}
	}

	// Map aliases to providers and models
	for _, alias := range aliases {
		if !alias.Enabled {
			continue
		}

		// Get the target model
		model, err := modelRepo.GetByID(ctx, alias.TargetModelID)
		if err != nil {
			continue // Skip invalid aliases
		}

		// If alias has a specific provider, use it; otherwise use model's default provider
		if alias.ProviderID != nil {
			newAliasToProvider[alias.Alias] = alias.ProviderID.String()
		} else if providerID, exists := newModelToProvider[model.ModelName]; exists {
			newAliasToProvider[alias.Alias] = providerID
		}

		newAliasToModel[alias.Alias] = model.ModelName
	}

	// Close old providers
	r.mu.Lock()
	for _, oldProvider := range r.providers {
		oldProvider.Close()
	}

	// Swap in new mappings
	r.providers = newProviders
	r.modelToProvider = newModelToProvider
	r.aliasToProvider = newAliasToProvider
	r.aliasToModel = newAliasToModel
	r.mu.Unlock()

	return nil
}

// Close closes all providers and stops the reload loop
func (r *ProviderRegistry) Close() error {
	// Stop reload loop
	close(r.stopCh)
	r.wg.Wait()

	// Close all providers
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, provider := range r.providers {
		if err := provider.Close(); err != nil {
			// Log error but continue closing others
			fmt.Printf("error closing provider %s: %v\n", provider.ID(), err)
		}
	}

	r.providers = make(map[string]Provider)
	r.modelToProvider = make(map[string]string)
	r.aliasToProvider = make(map[string]string)
	r.aliasToModel = make(map[string]string)

	return nil
}

// reloadLoop periodically reloads providers from the database
func (r *ProviderRegistry) reloadLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := r.Reload(ctx); err != nil {
				fmt.Printf("error reloading providers: %v\n", err)
			}
			cancel()

		case <-r.stopCh:
			return
		}
	}
}

// matchesLiteLLMProvider checks if a provider type matches a litellm provider string
func matchesLiteLLMProvider(providerType, liteLLMProvider string) bool {
	// Simple mapping - you can expand this based on your needs
	switch providerType {
	case "openai":
		return liteLLMProvider == "openai"
	case "vertexai":
		return liteLLMProvider == "vertex_ai" || liteLLMProvider == "vertexai"
	case "bedrock":
		return liteLLMProvider == "bedrock" || liteLLMProvider == "aws_bedrock"
	default:
		return providerType == liteLLMProvider
	}
}
