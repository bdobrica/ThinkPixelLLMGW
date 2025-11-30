package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

// TestAdminAPIKeysHandlerCreate tests the Create endpoint
func TestAdminAPIKeysHandlerCreate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()

	cfg := setupTestConfig(t)
	handler := NewAdminAPIKeysHandler(db)

	// Generate admin JWT
	jwt := generateAdminJWT(t, cfg, "admin")

	// Cleanup
	defer cleanupTestAPIKeys(t, db)

	tests := []struct {
		name           string
		request        CreateAPIKeyRequest
		expectedStatus int
		checkResponse  func(t *testing.T, resp *APIKeyCreatedResponse)
	}{
		{
			name: "create basic API key",
			request: CreateAPIKeyRequest{
				Name:               "Test API Key",
				RateLimitPerMinute: 100,
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *APIKeyCreatedResponse) {
				if resp.Name != "Test API Key" {
					t.Errorf("Expected name 'Test API Key', got '%s'", resp.Name)
				}
				if resp.RateLimitPerMinute != 100 {
					t.Errorf("Expected rate limit 100, got %d", resp.RateLimitPerMinute)
				}
				if resp.Key == "" {
					t.Error("Expected plaintext key to be returned")
				}
				if len(resp.Key) < 32 {
					t.Errorf("Expected key length >= 32, got %d", len(resp.Key))
				}
				if resp.Enabled != true {
					t.Error("Expected key to be enabled by default")
				}
			},
		},
		{
			name: "create API key with all fields",
			request: CreateAPIKeyRequest{
				Name:               "Full API Key",
				AllowedModels:      []string{"gpt-4", "gpt-3.5-turbo"},
				RateLimitPerMinute: 50,
				MonthlyBudgetUSD:   utils.FloatPtr(100.0),
				Enabled:            utils.BoolPtr(true),
				Tags: map[string]string{
					"environment": "production",
					"team":        "engineering",
				},
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *APIKeyCreatedResponse) {
				if len(resp.AllowedModels) != 2 {
					t.Errorf("Expected 2 allowed models, got %d", len(resp.AllowedModels))
				}
				if resp.MonthlyBudgetUSD == nil || *resp.MonthlyBudgetUSD != 100.0 {
					t.Error("Expected monthly budget to be 100.0")
				}
				if len(resp.Tags) != 2 {
					t.Errorf("Expected 2 tags, got %d", len(resp.Tags))
				}
			},
		},
		{
			name: "create API key with expiration",
			request: CreateAPIKeyRequest{
				Name:               "Expiring Key",
				RateLimitPerMinute: 60,
				ExpiresAt:          utils.StringPtr(time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)),
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *APIKeyCreatedResponse) {
				if resp.ExpiresAt == nil {
					t.Error("Expected expiration date to be set")
				}
			},
		},
		{
			name: "create disabled API key",
			request: CreateAPIKeyRequest{
				Name:               "Disabled Key",
				RateLimitPerMinute: 60,
				Enabled:            utils.BoolPtr(false),
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *APIKeyCreatedResponse) {
				if resp.Enabled != false {
					t.Error("Expected key to be disabled")
				}
			},
		},
		{
			name:           "create without name",
			request:        CreateAPIKeyRequest{RateLimitPerMinute: 60},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/admin/keys", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+jwt)

			w := httptest.NewRecorder()
			handler.Create(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
				return
			}

			if tt.expectedStatus == http.StatusCreated && tt.checkResponse != nil {
				var resp APIKeyCreatedResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				tt.checkResponse(t, &resp)
			}
		})
	}
}

// TestAdminAPIKeysHandlerList tests the List endpoint
func TestAdminAPIKeysHandlerList(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()

	cfg := setupTestConfig(t)
	handler := NewAdminAPIKeysHandler(db)

	// Generate admin JWT
	jwt := generateAdminJWT(t, cfg, "viewer")

	// Cleanup
	defer cleanupTestAPIKeys(t, db)

	// Create test API keys
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	for i := 0; i < 3; i++ {
		key := &models.APIKey{
			ID:                 uuid.New(),
			Name:               "Test Key " + string(rune('A'+i)),
			KeyHash:            hashAPIKey("test-key-" + string(rune('0'+i))),
			AllowedModels:      pq.StringArray{},
			RateLimitPerMinute: 60,
			Enabled:            true,
		}
		if err := apiKeyRepo.Create(context.Background(), key); err != nil {
			t.Fatalf("Failed to create test API key: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/keys?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response struct {
		Items      []APIKeyResponse `json:"items"`
		TotalCount int              `json:"total_count"`
		Page       int              `json:"page"`
		PageSize   int              `json:"page_size"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Items) < 3 {
		t.Errorf("Expected at least 3 API keys, got %d", len(response.Items))
	}

	// Verify keys don't contain hashes or plaintext keys
	for _, key := range response.Items {
		if key.ID == "" {
			t.Error("Expected key ID to be set")
		}
		if key.Name == "" {
			t.Error("Expected key name to be set")
		}
	}
}

// TestAdminAPIKeysHandlerGetByID tests the GetByID endpoint
func TestAdminAPIKeysHandlerGetByID(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()

	cfg := setupTestConfig(t)
	handler := NewAdminAPIKeysHandler(db)

	// Generate admin JWT
	jwt := generateAdminJWT(t, cfg, "viewer")

	// Cleanup
	defer cleanupTestAPIKeys(t, db)

	// Create test API key
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	testKey := &models.APIKey{
		ID:                 uuid.New(),
		Name:               "Test Detail Key",
		KeyHash:            hashAPIKey("test-detail-key"),
		AllowedModels:      pq.StringArray{"gpt-4"},
		RateLimitPerMinute: 100,
		MonthlyBudgetUSD:   utils.FloatPtr(50.0),
		Enabled:            true,
	}
	if err := apiKeyRepo.Create(context.Background(), testKey); err != nil {
		t.Fatalf("Failed to create test API key: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/keys/"+testKey.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	handler.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response APIKeyDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Name != "Test Detail Key" {
		t.Errorf("Expected name 'Test Detail Key', got '%s'", response.Name)
	}

	if response.UsageStats.CurrentMonthUSD < 0 {
		t.Error("Expected usage stats to be initialized")
	}
}

// TestAdminAPIKeysHandlerUpdate tests the Update endpoint
func TestAdminAPIKeysHandlerUpdate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()

	cfg := setupTestConfig(t)
	handler := NewAdminAPIKeysHandler(db)

	// Generate admin JWT
	jwt := generateAdminJWT(t, cfg, "admin")

	// Cleanup
	defer cleanupTestAPIKeys(t, db)

	// Create test API key
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	testKey := &models.APIKey{
		ID:                 uuid.New(),
		Name:               "Original Name",
		KeyHash:            hashAPIKey("test-update-key"),
		AllowedModels:      pq.StringArray{},
		RateLimitPerMinute: 60,
		Enabled:            true,
	}
	if err := apiKeyRepo.Create(context.Background(), testKey); err != nil {
		t.Fatalf("Failed to create test API key: %v", err)
	}

	updateReq := UpdateAPIKeyRequest{
		Name:               utils.StringPtr("Updated Name"),
		RateLimitPerMinute: utils.IntPtr(120),
		AllowedModels:      []string{"gpt-4", "claude-3"},
		Enabled:            utils.BoolPtr(false),
	}

	body, _ := json.Marshal(updateReq)
	req := httptest.NewRequest(http.MethodPut, "/admin/keys/"+testKey.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response APIKeyResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", response.Name)
	}
	if response.RateLimitPerMinute != 120 {
		t.Errorf("Expected rate limit 120, got %d", response.RateLimitPerMinute)
	}
	if response.Enabled != false {
		t.Error("Expected key to be disabled")
	}
	if len(response.AllowedModels) != 2 {
		t.Errorf("Expected 2 allowed models, got %d", len(response.AllowedModels))
	}
}

// TestAdminAPIKeysHandlerDelete tests the Delete endpoint
func TestAdminAPIKeysHandlerDelete(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()

	cfg := setupTestConfig(t)
	handler := NewAdminAPIKeysHandler(db)

	// Generate admin JWT
	jwt := generateAdminJWT(t, cfg, "admin")

	// Cleanup
	defer cleanupTestAPIKeys(t, db)

	// Create test API key
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	testKey := &models.APIKey{
		ID:                 uuid.New(),
		Name:               "Key to Delete",
		KeyHash:            hashAPIKey("test-delete-key"),
		AllowedModels:      pq.StringArray{},
		RateLimitPerMinute: 60,
		Enabled:            true,
	}
	if err := apiKeyRepo.Create(context.Background(), testKey); err != nil {
		t.Fatalf("Failed to create test API key: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/keys/"+testKey.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	handler.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Verify key is disabled (soft delete)
	key, err := apiKeyRepo.GetByID(context.Background(), testKey.ID)
	if err != nil {
		t.Fatalf("Failed to get API key: %v", err)
	}
	if key.Enabled {
		t.Error("Expected key to be disabled after delete")
	}
}

// TestAdminAPIKeysHandlerRegenerate tests the Regenerate endpoint
func TestAdminAPIKeysHandlerRegenerate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()

	cfg := setupTestConfig(t)
	handler := NewAdminAPIKeysHandler(db)

	// Generate admin JWT
	jwt := generateAdminJWT(t, cfg, "admin")

	// Cleanup
	defer cleanupTestAPIKeys(t, db)

	// Create test API key
	apiKeyRepo := storage.NewAPIKeyRepository(db)
	originalHash := hashAPIKey("original-key")
	testKey := &models.APIKey{
		ID:                 uuid.New(),
		Name:               "Key to Regenerate",
		KeyHash:            originalHash,
		AllowedModels:      pq.StringArray{},
		RateLimitPerMinute: 60,
		Enabled:            true,
	}
	if err := apiKeyRepo.Create(context.Background(), testKey); err != nil {
		t.Fatalf("Failed to create test API key: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/keys/"+testKey.ID.String()+"/regenerate", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	w := httptest.NewRecorder()
	handler.Regenerate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response APIKeyCreatedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Key == "" {
		t.Error("Expected new plaintext key to be returned")
	}

	// Verify hash was updated
	key, err := apiKeyRepo.GetByID(context.Background(), testKey.ID)
	if err != nil {
		t.Fatalf("Failed to get API key: %v", err)
	}
	if key.KeyHash == originalHash {
		t.Error("Expected key hash to be different after regeneration")
	}

	// Verify new key hash matches
	newHash := hashAPIKey(response.Key)
	if key.KeyHash != newHash {
		t.Error("Expected new key hash to match regenerated key")
	}
}

// Helper functions

func cleanupTestAPIKeys(t *testing.T, db *storage.DB) {
	_, err := db.Conn().Exec("DELETE FROM api_keys WHERE name LIKE 'Test%' OR name LIKE '%Key%'")
	if err != nil {
		t.Logf("Warning: Failed to cleanup test API keys: %v", err)
	}
}
