package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestAPIKey_AllowsModel(t *testing.T) {
	tests := []struct {
		name          string
		allowedModels []string
		testModel     string
		expected      bool
	}{
		{
			name:          "empty list allows all",
			allowedModels: []string{},
			testModel:     "gpt-4",
			expected:      true,
		},
		{
			name:          "nil list allows all",
			allowedModels: nil,
			testModel:     "claude-3",
			expected:      true,
		},
		{
			name:          "model in list",
			allowedModels: []string{"gpt-4", "gpt-3.5-turbo"},
			testModel:     "gpt-4",
			expected:      true,
		},
		{
			name:          "model not in list",
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
		{
			name:          "case sensitive",
			allowedModels: []string{"gpt-4"},
			testModel:     "GPT-4",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{
				AllowedModels: pq.StringArray(tt.allowedModels),
			}

			result := key.AllowsModel(tt.testModel)
			if result != tt.expected {
				t.Errorf("AllowsModel(%q) = %v, want %v", tt.testModel, result, tt.expected)
			}
		})
	}
}

func TestAPIKey_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "nil expiration never expires",
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "past expiration is expired",
			expiresAt: &past,
			expected:  true,
		},
		{
			name:      "future expiration not expired",
			expiresAt: &future,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{
				ExpiresAt: tt.expiresAt,
			}

			result := key.IsExpired()
			if result != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAPIKey_IsValid(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name      string
		enabled   bool
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "enabled and not expired",
			enabled:   true,
			expiresAt: nil,
			expected:  true,
		},
		{
			name:      "enabled with future expiration",
			enabled:   true,
			expiresAt: &future,
			expected:  true,
		},
		{
			name:      "disabled",
			enabled:   false,
			expiresAt: nil,
			expected:  false,
		},
		{
			name:      "enabled but expired",
			enabled:   true,
			expiresAt: &past,
			expected:  false,
		},
		{
			name:      "disabled and expired",
			enabled:   false,
			expiresAt: &past,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &APIKey{
				Enabled:   tt.enabled,
				ExpiresAt: tt.expiresAt,
			}

			result := key.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAPIKey_FullLifecycle(t *testing.T) {
	// Create a new API key
	keyID := uuid.New()
	now := time.Now()
	budget := 100.0
	future := now.Add(24 * time.Hour)

	key := &APIKey{
		ID:                 keyID,
		Name:               "Test API Key",
		KeyHash:            "hash123",
		AllowedModels:      pq.StringArray{"gpt-4", "gpt-3.5-turbo"},
		RateLimitPerMinute: 100,
		MonthlyBudgetUSD:   &budget,
		Enabled:            true,
		ExpiresAt:          &future,
		CreatedAt:          now,
		UpdatedAt:          now,
		Tags:               map[string]string{"env": "test", "team": "engineering"},
	}

	// Test initial state
	if !key.IsValid() {
		t.Error("Newly created key should be valid")
	}

	if !key.AllowsModel("gpt-4") {
		t.Error("Key should allow gpt-4")
	}

	if key.AllowsModel("claude-3") {
		t.Error("Key should not allow claude-3")
	}

	// Disable the key
	key.Enabled = false
	if key.IsValid() {
		t.Error("Disabled key should not be valid")
	}

	// Re-enable but expire
	key.Enabled = true
	past := now.Add(-1 * time.Hour)
	key.ExpiresAt = &past
	if key.IsValid() {
		t.Error("Expired key should not be valid")
	}

	// Remove expiration
	key.ExpiresAt = nil
	if !key.IsValid() {
		t.Error("Key without expiration should be valid when enabled")
	}
}

func TestAPIKey_TagsHandling(t *testing.T) {
	key := &APIKey{
		ID:   uuid.New(),
		Name: "Tagged Key",
		Tags: map[string]string{
			"environment": "production",
			"owner":       "team-a",
			"cost-center": "eng-123",
		},
	}

	if len(key.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(key.Tags))
	}

	if key.Tags["environment"] != "production" {
		t.Errorf("Expected environment=production, got %s", key.Tags["environment"])
	}

	// Test nil tags
	keyNoTags := &APIKey{
		ID:   uuid.New(),
		Name: "No Tags Key",
		Tags: nil,
	}

	if len(keyNoTags.Tags) > 0 {
		t.Error("Expected nil or empty tags")
	}
}

func TestAPIKey_RateLimitAndBudget(t *testing.T) {
	t.Run("with budget", func(t *testing.T) {
		budget := 50.75
		key := &APIKey{
			ID:                 uuid.New(),
			MonthlyBudgetUSD:   &budget,
			RateLimitPerMinute: 60,
		}

		if key.MonthlyBudgetUSD == nil {
			t.Error("MonthlyBudgetUSD should not be nil")
		}
		if *key.MonthlyBudgetUSD != 50.75 {
			t.Errorf("MonthlyBudgetUSD = %f, want 50.75", *key.MonthlyBudgetUSD)
		}
		if key.RateLimitPerMinute != 60 {
			t.Errorf("RateLimitPerMinute = %d, want 60", key.RateLimitPerMinute)
		}
	})

	t.Run("unlimited budget", func(t *testing.T) {
		key := &APIKey{
			ID:                 uuid.New(),
			MonthlyBudgetUSD:   nil,
			RateLimitPerMinute: 100,
		}

		if key.MonthlyBudgetUSD != nil {
			t.Error("MonthlyBudgetUSD should be nil for unlimited")
		}
	})
}
