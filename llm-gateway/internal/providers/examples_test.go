package providers_test

import (
	"context"
	"testing"
	"time"

	"gateway/internal/providers"
)

// Example usage of the provider system
func ExampleProviderRegistry() {
	// This example shows how to use the provider registry in your application
	
	// 1. Create registry (in real app, you'd use actual DB and encryption)
	registry, err := providers.NewProviderRegistry(providers.RegistryConfig{
		DB:             nil, // Replace with actual *storage.DB
		Encryption:     nil, // Replace with actual *storage.Encryption
		ReloadInterval: 5 * time.Minute,
	})
	if err != nil {
		panic(err)
	}
	defer registry.Close()
	
	// 2. Resolve a model
	ctx := context.Background()
	provider, modelName, err := registry.ResolveModel(ctx, "gpt-4")
	if err != nil {
		panic(err)
	}
	
	// 3. Send a chat request
	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Model: modelName,
		Payload: map[string]any{
			"messages": []map[string]string{
				{"role": "user", "content": "Hello, world!"},
			},
			"temperature": 0.7,
		},
		Stream: false,
	})
	if err != nil {
		panic(err)
	}
	
	// 4. Handle response
	if resp.StatusCode == 200 {
		// Success - process response
		_ = resp.Body
		_ = resp.CostUSD
		_ = resp.ProviderLatency
	}
}

// Example of streaming chat
func ExampleProviderStreaming() {
	var registry *providers.ProviderRegistry // initialized elsewhere
	
	ctx := context.Background()
	provider, modelName, _ := registry.ResolveModel(ctx, "gpt-4")
	
	// Send streaming request
	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Model: modelName,
		Payload: map[string]any{
			"messages": []map[string]string{
				{"role": "user", "content": "Tell me a story"},
			},
			"stream": true,
		},
		Stream: true,
	})
	if err != nil {
		panic(err)
	}
	
	// Read stream
	if resp.Stream != nil {
		defer resp.Stream.Close()
		
		reader := providers.NewStreamReader(resp.Stream)
		defer reader.Close()
		
		for {
			event, err := reader.Read()
			if event.Done {
				break
			}
			if err != nil {
				break
			}
			
			// Process streaming data
			_ = event.Data
		}
	}
}

// Example of adding a custom provider
func ExampleCustomProvider() {
	// 1. Define your provider type
	type MyCustomProvider struct {
		id   string
		name string
	}
	
	// 2. Implement the Provider interface
	func (p *MyCustomProvider) ID() string { return p.id }
	func (p *MyCustomProvider) Name() string { return p.name }
	func (p *MyCustomProvider) Type() string { return "mycustom" }
	
	func (p *MyCustomProvider) Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
		// Your implementation here
		return &providers.ChatResponse{
			StatusCode: 200,
			Body:       []byte(`{"message": "Hello from custom provider"}`),
		}, nil
	}
	
	func (p *MyCustomProvider) ValidateCredentials(ctx context.Context) error {
		// Validate your credentials
		return nil
	}
	
	func (p *MyCustomProvider) Close() error {
		// Cleanup resources
		return nil
	}
	
	// 3. Create a constructor
	func NewMyCustomProvider(config providers.ProviderConfig) (providers.Provider, error) {
		return &MyCustomProvider{
			id:   config.ID,
			name: config.Name,
		}, nil
	}
	
	// 4. Register with factory
	factory := providers.NewProviderFactory()
	factory.Register("mycustom", NewMyCustomProvider)
}

// Mock test for provider resolution
func TestProviderResolution(t *testing.T) {
	// This is a conceptual test - you'd need actual DB setup
	t.Skip("Requires database setup")
	
	// Setup
	ctx := context.Background()
	registry, _ := providers.NewProviderRegistry(providers.RegistryConfig{
		// DB config
	})
	defer registry.Close()
	
	// Test model resolution
	provider, modelName, err := registry.ResolveModel(ctx, "gpt-4")
	if err != nil {
		t.Fatalf("Failed to resolve model: %v", err)
	}
	
	if provider == nil {
		t.Fatal("Provider is nil")
	}
	
	if modelName != "gpt-4" {
		t.Errorf("Expected model name 'gpt-4', got '%s'", modelName)
	}
	
	// Test alias resolution
	provider, modelName, err = registry.ResolveModel(ctx, "my-custom-alias")
	if err != nil {
		t.Fatalf("Failed to resolve alias: %v", err)
	}
	
	if provider == nil {
		t.Fatal("Provider is nil for alias")
	}
}

// Mock test for OpenAI provider
func TestOpenAIProvider(t *testing.T) {
	t.Skip("Requires actual OpenAI API key")
	
	// Create provider
	provider, err := providers.NewOpenAIProvider(providers.ProviderConfig{
		ID:   "test-openai",
		Name: "Test OpenAI",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "sk-test-key",
		},
		Config: map[string]any{
			"base_url": "https://api.openai.com/v1",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()
	
	// Test chat
	ctx := context.Background()
	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Model: "gpt-3.5-turbo",
		Payload: map[string]any{
			"messages": []map[string]string{
				{"role": "user", "content": "Say 'test'"},
			},
			"max_tokens": 10,
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	if resp.CostUSD <= 0 {
		t.Error("Expected positive cost")
	}
}

// Benchmark model resolution
func BenchmarkModelResolution(b *testing.B) {
	b.Skip("Requires database setup")
	
	ctx := context.Background()
	registry, _ := providers.NewProviderRegistry(providers.RegistryConfig{
		// DB config
	})
	defer registry.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = registry.ResolveModel(ctx, "gpt-4")
	}
}

// Example of handling errors
func ExampleErrorHandling() {
	var registry *providers.ProviderRegistry // initialized elsewhere
	ctx := context.Background()
	
	// Handle model not found
	provider, modelName, err := registry.ResolveModel(ctx, "unknown-model")
	if err != nil {
		// Log error: "model or alias not found: unknown-model"
		return
	}
	
	// Handle provider request failure
	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Model:   modelName,
		Payload: map[string]any{},
	})
	if err != nil {
		// Log error: network error, timeout, etc.
		return
	}
	
	// Handle provider error response
	if resp.StatusCode != 200 {
		// Log error from provider
		_ = string(resp.Body)
		return
	}
	
	// Success
	_ = resp.Body
}
