package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProviderType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{"OpenAI", ProviderTypeOpenAI, "openai"},
		{"VertexAI", ProviderTypeVertexAI, "vertexai"},
		{"Bedrock", ProviderTypeBedrock, "bedrock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("ProviderType = %s, want %s", tt.provider, tt.expected)
			}
		})
	}
}

func TestProvider_Creation(t *testing.T) {
	now := time.Now()
	providerID := uuid.New()

	provider := &Provider{
		ID:                   providerID,
		Name:                 "test-openai",
		DisplayName:          "Test OpenAI Provider",
		ProviderType:         string(ProviderTypeOpenAI),
		EncryptedCredentials: JSONB{"api_key": "encrypted-key"},
		Config:               JSONB{"base_url": "https://api.openai.com/v1"},
		Enabled:              true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if provider.ID != providerID {
		t.Errorf("Provider ID = %v, want %v", provider.ID, providerID)
	}
	if provider.Name != "test-openai" {
		t.Errorf("Provider Name = %s, want test-openai", provider.Name)
	}
	if provider.DisplayName != "Test OpenAI Provider" {
		t.Errorf("Provider DisplayName = %s, want Test OpenAI Provider", provider.DisplayName)
	}
	if provider.ProviderType != "openai" {
		t.Errorf("Provider Type = %s, want openai", provider.ProviderType)
	}
	if !provider.Enabled {
		t.Error("Provider should be enabled")
	}
}

func TestProvider_Disabled(t *testing.T) {
	provider := &Provider{
		ID:           uuid.New(),
		Name:         "disabled-provider",
		DisplayName:  "Disabled Provider",
		ProviderType: string(ProviderTypeBedrock),
		Enabled:      false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if provider.Enabled {
		t.Error("Provider should be disabled")
	}
}

func TestProvider_MultipleProviderTypes(t *testing.T) {
	providers := []Provider{
		{
			ID:           uuid.New(),
			Name:         "openai-1",
			ProviderType: string(ProviderTypeOpenAI),
			Enabled:      true,
		},
		{
			ID:           uuid.New(),
			Name:         "vertexai-1",
			ProviderType: string(ProviderTypeVertexAI),
			Enabled:      true,
		},
		{
			ID:           uuid.New(),
			Name:         "bedrock-1",
			ProviderType: string(ProviderTypeBedrock),
			Enabled:      true,
		},
	}

	expectedTypes := map[string]bool{
		"openai":   true,
		"vertexai": true,
		"bedrock":  true,
	}

	for _, provider := range providers {
		if !expectedTypes[provider.ProviderType] {
			t.Errorf("Unexpected provider type: %s", provider.ProviderType)
		}
	}
}

func TestProvider_CredentialsAndConfig(t *testing.T) {
	t.Run("OpenAI credentials", func(t *testing.T) {
		provider := &Provider{
			ID:                   uuid.New(),
			Name:                 "openai-test",
			ProviderType:         string(ProviderTypeOpenAI),
			EncryptedCredentials: JSONB{"api_key": "sk-test-key"},
			Config:               JSONB{"base_url": "https://api.openai.com/v1", "timeout": 30},
			Enabled:              true,
		}

		if len(provider.EncryptedCredentials) == 0 {
			t.Error("EncryptedCredentials should not be empty")
		}
		if len(provider.Config) == 0 {
			t.Error("Config should not be empty")
		}
	})

	t.Run("VertexAI credentials", func(t *testing.T) {
		provider := &Provider{
			ID:           uuid.New(),
			Name:         "vertexai-test",
			ProviderType: string(ProviderTypeVertexAI),
			EncryptedCredentials: JSONB{
				"project_id":       "my-project",
				"location":         "us-central1",
				"credentials_json": "base64-encoded-json",
			},
			Config:  JSONB{"endpoint": "us-central1-aiplatform.googleapis.com"},
			Enabled: true,
		}

		if len(provider.EncryptedCredentials) == 0 {
			t.Error("EncryptedCredentials should not be empty")
		}
	})

	t.Run("Bedrock credentials", func(t *testing.T) {
		provider := &Provider{
			ID:           uuid.New(),
			Name:         "bedrock-test",
			ProviderType: string(ProviderTypeBedrock),
			EncryptedCredentials: JSONB{
				"aws_access_key_id":     "AKIA...",
				"aws_secret_access_key": "secret",
				"aws_region":            "us-east-1",
			},
			Config:  JSONB{"region": "us-east-1"},
			Enabled: true,
		}

		if len(provider.EncryptedCredentials) == 0 {
			t.Error("EncryptedCredentials should not be empty")
		}
	})
}

func TestProvider_Timestamps(t *testing.T) {
	now := time.Now()
	provider := &Provider{
		ID:        uuid.New(),
		Name:      "timestamped-provider",
		CreatedAt: now,
		UpdatedAt: now,
		Enabled:   true,
	}

	if provider.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if provider.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	// Simulate an update
	later := now.Add(1 * time.Hour)
	provider.UpdatedAt = later
	provider.DisplayName = "Updated Provider"

	if !provider.UpdatedAt.After(provider.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt")
	}
}
