package providers

import "testing"

func TestProviderIdentityConformance(t *testing.T) {
	tests := []struct {
		name, providerType string
		create             func() (Provider, error)
	}{
		{"OpenAI", "openai", func() (Provider, error) {
			return NewOpenAIProvider(ProviderConfig{ID: "id", Name: "display", Credentials: map[string]string{"api_key": "key"}, Config: map[string]any{}})
		}},
		{"Vertex AI", "vertexai", func() (Provider, error) {
			return NewVertexAIProvider(ProviderConfig{ID: "id", Name: "display", Credentials: map[string]string{"access_token": "token"}, Config: map[string]any{"project_id": "project"}})
		}},
		{"Bedrock", "bedrock", func() (Provider, error) {
			return NewBedrockProvider(ProviderConfig{ID: "id", Name: "display", Credentials: map[string]string{"access_key_id": "key", "secret_access_key": "secret"}, Config: map[string]any{"region": "us-east-1"}})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := test.create()
			if err != nil {
				t.Fatal(err)
			}
			if provider.ID() != "id" || provider.Name() != "display" || provider.Type() != test.providerType {
				t.Fatalf("provider identity contract failed: id=%q name=%q type=%q", provider.ID(), provider.Name(), provider.Type())
			}
			if err := provider.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
