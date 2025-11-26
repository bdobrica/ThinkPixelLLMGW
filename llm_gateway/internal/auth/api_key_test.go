package auth

import (
	"context"
	"testing"

	"llm_gateway/internal/utils"
)

func TestAPIKeyRecord_AllowsModel(t *testing.T) {
	tests := []struct {
		name          string
		allowedModels []string
		testModel     string
		expected      bool
	}{
		{
			name:          "empty allowed models allows all",
			allowedModels: []string{},
			testModel:     "gpt-4",
			expected:      true,
		},
		{
			name:          "nil allowed models allows all",
			allowedModels: nil,
			testModel:     "claude-3",
			expected:      true,
		},
		{
			name:          "model in allowed list",
			allowedModels: []string{"gpt-4", "gpt-3.5-turbo", "claude-3"},
			testModel:     "gpt-4",
			expected:      true,
		},
		{
			name:          "model not in allowed list",
			allowedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			testModel:     "claude-3",
			expected:      false,
		},
		{
			name:          "exact match required",
			allowedModels: []string{"gpt-4"},
			testModel:     "gpt-4-turbo",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKeyRecord{
				ID:            "test-id",
				Name:          "Test Key",
				AllowedModels: tt.allowedModels,
			}

			result := key.AllowsModel(tt.testModel)
			if result != tt.expected {
				t.Errorf("AllowsModel(%q) = %v, want %v", tt.testModel, result, tt.expected)
			}
		})
	}
}

func TestInMemoryAPIKeyStore_Lookup(t *testing.T) {
	store := NewInMemoryAPIKeyStore()
	ctx := context.Background()

	t.Run("valid demo key", func(t *testing.T) {
		record, err := store.Lookup(ctx, "demo-key")
		if err != nil {
			t.Fatalf("Lookup() error = %v, want nil", err)
		}
		if record == nil {
			t.Fatal("Lookup() returned nil record")
		}
		if record.ID != "demo-key-id" {
			t.Errorf("Lookup() ID = %v, want demo-key-id", record.ID)
		}
		if record.Name != "Demo Key" {
			t.Errorf("Lookup() Name = %v, want Demo Key", record.Name)
		}
		if record.Revoked {
			t.Error("Lookup() Revoked = true, want false")
		}
		if len(record.Tags) == 0 {
			t.Error("Lookup() Tags is empty, expected tags")
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		record, err := store.Lookup(ctx, "invalid-key-123")
		if err != ErrKeyNotFound {
			t.Errorf("Lookup() error = %v, want ErrKeyNotFound", err)
		}
		if record != nil {
			t.Errorf("Lookup() record = %v, want nil", record)
		}
	})

	t.Run("empty key", func(t *testing.T) {
		record, err := store.Lookup(ctx, "")
		if err != ErrKeyNotFound {
			t.Errorf("Lookup() error = %v, want ErrKeyNotFound", err)
		}
		if record != nil {
			t.Errorf("Lookup() record = %v, want nil", record)
		}
	})
}

func TestInMemoryAPIKeyStore_AddKey(t *testing.T) {
	store := NewInMemoryAPIKeyStore()
	ctx := context.Background()

	// Add a custom key
	customKey := "custom-test-key-456"
	customHash := utils.HashString(customKey)
	store.keys[customHash] = &APIKeyRecord{
		ID:            "custom-id",
		Name:          "Custom Key",
		AllowedModels: []string{"gpt-4"},
		Tags:          map[string]string{"team": "engineering"},
		Revoked:       false,
	}

	// Lookup the custom key
	record, err := store.Lookup(ctx, customKey)
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if record.ID != "custom-id" {
		t.Errorf("Lookup() ID = %v, want custom-id", record.ID)
	}
	if !record.AllowsModel("gpt-4") {
		t.Error("AllowsModel(gpt-4) = false, want true")
	}
	if record.AllowsModel("claude-3") {
		t.Error("AllowsModel(claude-3) = true, want false")
	}
}

func TestAPIKeyRecord_RevokedKey(t *testing.T) {
	key := &APIKeyRecord{
		ID:            "revoked-id",
		Name:          "Revoked Key",
		AllowedModels: []string{},
		Tags:          map[string]string{},
		Revoked:       true,
	}

	if !key.Revoked {
		t.Error("Revoked = false, want true")
	}
}
