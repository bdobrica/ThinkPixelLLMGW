package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"

	"github.com/google/uuid"
)

var registryLogger = utils.NewLogger("provider-registry", utils.Info)

// ProviderRegistry manages all provider instances and resolves models to providers
type ProviderRegistry struct {
	factory    Factory
	db         *storage.DB
	encryption *storage.Encryption

	mu               sync.RWMutex
	providers        map[string]*managedProvider // provider ID -> Provider instance
	modelToProvider  map[string]string           // model name -> provider ID
	aliasToProvider  map[string]string           // alias -> provider ID
	aliasToModel     map[string]string           // alias -> actual model name
	closeGracePeriod time.Duration

	reloadInterval time.Duration
	requestTimeout time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

type managedProvider struct {
	Provider
	closeOnce sync.Once
	closeErr  error
}

func newManagedProvider(provider Provider) *managedProvider {
	return &managedProvider{Provider: provider}
}

func (p *managedProvider) Close() error {
	p.closeOnce.Do(func() {
		p.closeErr = p.Provider.Close()
	})
	return p.closeErr
}

// RegistryConfig holds configuration for the provider registry
type RegistryConfig struct {
	Factory        Factory
	DB             *storage.DB
	Encryption     *storage.Encryption
	ReloadInterval time.Duration // how often to reload providers from DB (0 = no auto-reload)
	RequestTimeout time.Duration // default timeout injected into providers when not explicitly configured
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
		factory:          config.Factory,
		db:               config.DB,
		encryption:       config.Encryption,
		providers:        make(map[string]*managedProvider),
		modelToProvider:  make(map[string]string),
		aliasToProvider:  make(map[string]string),
		aliasToModel:     make(map[string]string),
		closeGracePeriod: providerCloseGracePeriod(config.RequestTimeout),
		reloadInterval:   config.ReloadInterval,
		requestTimeout:   config.RequestTimeout,
		stopCh:           make(chan struct{}),
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

// ResolveModelWithDetails resolves a model name or alias to a provider, model name, and full model details
// This includes pricing components needed for accurate cost calculation
func (r *ProviderRegistry) ResolveModelWithDetails(ctx context.Context, modelNameOrAlias string) (Provider, string, interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var actualModelName string
	var providerID string

	// First check if it's an alias
	if pID, exists := r.aliasToProvider[modelNameOrAlias]; exists {
		providerID = pID
		actualModelName = r.aliasToModel[modelNameOrAlias]
	} else if pID, exists := r.modelToProvider[modelNameOrAlias]; exists {
		// It's a direct model name
		providerID = pID
		actualModelName = modelNameOrAlias
	} else {
		return nil, "", nil, fmt.Errorf("model or alias not found: %s", modelNameOrAlias)
	}

	// Get the provider
	provider, ok := r.providers[providerID]
	if !ok {
		return nil, "", nil, fmt.Errorf("provider %s not found", providerID)
	}

	// Get full model details with pricing components
	modelRepo := storage.NewModelRepository(r.db)
	model, err := modelRepo.GetByName(ctx, actualModelName)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to get model details: %w", err)
	}

	// Wrap in ModelWithDetails
	modelDetails := &storage.ModelWithDetails{
		Model:             model,
		PricingComponents: model.PricingComponents,
	}

	return provider, actualModelName, modelDetails, nil
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
	models, err := modelRepo.List(ctx, 10000, 0) // Get all models (with a high limit)
	if err != nil {
		return fmt.Errorf("failed to load models from database: %w", err)
	}

	// Build new provider instances
	newProviders := make(map[string]*managedProvider)
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
			// EncryptedCredentials is JSONB, convert to map first
			encryptedMap := make(map[string]any)
			for k, v := range dbProvider.EncryptedCredentials {
				encryptedMap[k] = v
			}

			// Decrypt each credential value
			for key, val := range encryptedMap {
				if strVal, ok := val.(string); ok {
					decrypted, err := r.encryption.Decrypt(strVal)
					if err != nil {
						return fmt.Errorf("failed to decrypt credential '%s' for provider %s: %w", key, dbProvider.Name, err)
					}
					credentials[key] = string(decrypted)
				}
			}
		}

		// Parse config (already a JSONB map)
		config := make(map[string]any)
		if dbProvider.Config != nil {
			config = dbProvider.Config
		}
		if r.requestTimeout > 0 {
			if _, exists := config["request_timeout"]; !exists {
				config["request_timeout"] = r.requestTimeout.String()
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

		newProviders[dbProvider.ID.String()] = newManagedProvider(provider)
	}

	// Map models to providers
	for _, model := range models {
		// Find which provider(s) support this model by matching litellm_provider
		for _, dbProvider := range dbProviders {
			if !dbProvider.Enabled {
				continue
			}

			// Simple heuristic: match provider type to provider_id
			// In production, you might have a more sophisticated mapping
			if matchesLiteLLMProvider(dbProvider.ProviderType, model.ProviderID) {
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
		if alias.ProviderID != uuid.Nil {
			newAliasToProvider[alias.Alias] = alias.ProviderID.String()
		} else if providerID, exists := newModelToProvider[model.ModelName]; exists {
			newAliasToProvider[alias.Alias] = providerID
		}

		newAliasToModel[alias.Alias] = model.ModelName
	}

	var oldProviders []*managedProvider
	r.mu.Lock()
	for _, oldProvider := range r.providers {
		oldProviders = append(oldProviders, oldProvider)
	}

	// Swap in new mappings
	r.providers = newProviders
	r.modelToProvider = newModelToProvider
	r.aliasToProvider = newAliasToProvider
	r.aliasToModel = newAliasToModel
	r.mu.Unlock()

	r.retireProviders(oldProviders)

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
			registryLogger.Error("error closing provider", "provider_id", provider.ID(), "error", err)
		}
	}

	r.providers = make(map[string]*managedProvider)
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
				registryLogger.Error("error reloading providers", "error", err)
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

func (r *ProviderRegistry) retireProviders(providers []*managedProvider) {
	if len(providers) == 0 {
		return
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		if r.closeGracePeriod > 0 {
			timer := time.NewTimer(r.closeGracePeriod)
			defer timer.Stop()

			select {
			case <-timer.C:
			case <-r.stopCh:
			}
		}

		for _, provider := range providers {
			if err := provider.Close(); err != nil {
				registryLogger.Error("error closing retired provider", "provider_id", provider.ID(), "error", err)
			}
		}
	}()
}

func providerCloseGracePeriod(requestTimeout time.Duration) time.Duration {
	const (
		defaultRequestTimeout = 60 * time.Second
		closeJitter           = 5 * time.Second
	)

	if requestTimeout <= 0 {
		requestTimeout = defaultRequestTimeout
	}

	return requestTimeout + closeJitter
}
