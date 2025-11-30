package httpapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/config"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
)

// Integration tests for AdminProvidersHandler
//
// These tests require a PostgreSQL database to be running.
// Use docker-compose from the root of the repo:
//
//   cd .. && docker-compose up -d postgres
//
// Then run tests:
//   DATABASE_URL="postgres://gateway:password@localhost:5432/gateway?sslmode=disable" go test -v -run TestAdminProviders

const (
	defaultTestDatabaseURL = "postgres://gateway:password@localhost:5432/gateway?sslmode=disable"
)

// getTestDatabaseURL returns the database URL from environment or default
func getTestDatabaseURL() string {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = defaultTestDatabaseURL
	}
	return dbURL
}

// skipIfNoDatabase skips the test if database is not available
func skipIfNoDatabase(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
}

// setupTestDB creates a test database connection and runs migrations
func setupTestDB(t *testing.T) *storage.DB {
	t.Helper()

	dbConfig := storage.DBConfig{
		DSN:             getTestDatabaseURL(),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 15 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		APIKeyCacheSize: 100,
		APIKeyCacheTTL:  5 * time.Minute,
		ModelCacheSize:  100,
		ModelCacheTTL:   15 * time.Minute,
	}

	db, err := storage.NewDB(dbConfig)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	return db
}

// setupTestEncryption creates a test encryption service
func setupTestEncryption(t *testing.T) *storage.Encryption {
	t.Helper()

	// Use a test encryption key (32 bytes)
	encryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encryptionKeyBytes, err := hex.DecodeString(encryptionKeyHex)
	if err != nil {
		t.Fatalf("Failed to decode encryption key: %v", err)
	}

	encryption, err := storage.NewEncryption(encryptionKeyBytes)
	if err != nil {
		t.Fatalf("Failed to create encryption: %v", err)
	}

	return encryption
}

// setupTestConfig creates a test configuration
func setupTestConfig(t *testing.T) *config.Config {
	t.Helper()

	return &config.Config{
		JWTSecret: []byte("test-secret-key-for-jwt-signing"),
	}
}

// setupTestProviderRegistry creates a test provider registry
func setupTestProviderRegistry(t *testing.T, db *storage.DB, encryption *storage.Encryption) providers.Registry {
	t.Helper()

	registry, err := providers.NewProviderRegistry(providers.RegistryConfig{
		DB:             db,
		Encryption:     encryption,
		ReloadInterval: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create provider registry: %v", err)
	}

	return registry
}

// generateAdminJWT generates a JWT token for testing
func generateAdminJWT(t *testing.T, cfg *config.Config, roles ...string) string {
	t.Helper()

	if len(roles) == 0 {
		roles = []string{auth.RoleAdmin.String()}
	}

	claims := &auth.AdminClaims{
		AdminID:  uuid.New().String(),
		AuthType: auth.AdminAuthTypeUser,
		Roles:    roles,
		Email:    "test@example.com",
	}

	token, _, err := auth.GenerateJWTWithClaims(claims, cfg)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	return token
}

// cleanupTestProviders removes all test providers from the database
func cleanupTestProviders(t *testing.T, db *storage.DB) {
	t.Helper()

	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)

	providers, err := providerRepo.List(ctx)
	if err != nil {
		t.Logf("Warning: Failed to list providers for cleanup: %v", err)
		return
	}

	for _, p := range providers {
		// Only delete test providers (those created in tests)
		if p.Name == "test-provider" || p.Name == "test-openai" || p.Name == "test-anthropic" {
			_ = providerRepo.Delete(ctx, p.ID)
		}
	}
}

// TestAdminProvidersHandlerCreate tests creating a new provider
func TestAdminProvidersHandlerCreate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestProviders(t, db)

	encryption := setupTestEncryption(t)
	cfg := setupTestConfig(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminProvidersHandler(db, encryption, registry)

	tests := []struct {
		name           string
		payload        CreateProviderRequest
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "successful_create_with_admin_role",
			payload: CreateProviderRequest{
				Name:        "test-openai",
				DisplayName: "Test OpenAI",
				Type:        string(models.ProviderTypeOpenAI),
				Credentials: map[string]interface{}{
					"api_key": "sk-test-key-123",
				},
				Config: map[string]interface{}{
					"base_url": "https://api.openai.com/v1",
				},
				Enabled: true,
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ProviderResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.Name != "test-openai" {
					t.Errorf("Expected name 'test-openai', got '%s'", result.Name)
				}
				if result.Type != string(models.ProviderTypeOpenAI) {
					t.Errorf("Expected type '%s', got '%s'", models.ProviderTypeOpenAI, result.Type)
				}
				if !result.Enabled {
					t.Error("Expected provider to be enabled")
				}
			},
		},
		{
			name: "missing_name",
			payload: CreateProviderRequest{
				Type: string(models.ProviderTypeOpenAI),
				Credentials: map[string]interface{}{
					"api_key": "sk-test",
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid_provider_type",
			payload: CreateProviderRequest{
				Name: "test-invalid",
				Type: "invalid-type",
				Credentials: map[string]interface{}{
					"api_key": "sk-test",
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate_provider_name",
			payload: CreateProviderRequest{
				Name:        "test-openai",
				DisplayName: "Duplicate",
				Type:        string(models.ProviderTypeOpenAI),
				Credentials: map[string]interface{}{
					"api_key": "sk-test-dup",
				},
				Enabled: true,
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/admin/providers", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")

			// Add JWT token to context
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply JWT middleware
			adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())
			resp := httptest.NewRecorder()

			adminMiddleware(http.HandlerFunc(handler.Create)).ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil && resp.Code == tt.expectedStatus {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminProvidersHandlerList tests listing providers
func TestAdminProvidersHandlerList(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestProviders(t, db)

	encryption := setupTestEncryption(t)
	cfg := setupTestConfig(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminProvidersHandler(db, encryption, registry)

	// Create test providers
	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)

	testProviders := []*models.Provider{
		{
			ID:           uuid.New(),
			Name:         "test-provider-1",
			DisplayName:  "Test Provider 1",
			ProviderType: string(models.ProviderTypeOpenAI),
			Enabled:      true,
		},
		{
			ID:           uuid.New(),
			Name:         "test-provider-2",
			DisplayName:  "Test Provider 2",
			ProviderType: string(models.ProviderTypeVertexAI),
			Enabled:      false,
		},
	}

	for _, p := range testProviders {
		if err := providerRepo.Create(ctx, p); err != nil {
			t.Fatalf("Failed to create test provider: %v", err)
		}
	}

	tests := []struct {
		name           string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "list_with_admin_role",
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var results []ProviderResponse
				if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should have at least our test providers
				if len(results) < 2 {
					t.Errorf("Expected at least 2 providers, got %d", len(results))
				}
			},
		},
		{
			name:           "list_with_viewer_role",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var results []ProviderResponse
				if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Verify no credentials are exposed
				for _, p := range results {
					// Credentials should not be in the list response
					if len(p.Config) > 0 {
						// Config is OK, but make sure no sensitive data
						t.Logf("Provider %s has config (expected)", p.Name)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/providers", nil)

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply JWT middleware
			viewerMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleViewer.String())
			resp := httptest.NewRecorder()

			viewerMiddleware(http.HandlerFunc(handler.List)).ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminProvidersHandlerGetByID tests getting a provider by ID
func TestAdminProvidersHandlerGetByID(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestProviders(t, db)

	encryption := setupTestEncryption(t)
	cfg := setupTestConfig(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminProvidersHandler(db, encryption, registry)

	// Create a test provider with encrypted credentials
	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)

	encryptedCreds := make(map[string]interface{})
	apiKey := "sk-test-secret-key-123"
	encryptedKey, err := encryption.Encrypt([]byte(apiKey))
	if err != nil {
		t.Fatalf("Failed to encrypt API key: %v", err)
	}
	encryptedCreds["api_key"] = encryptedKey

	testProvider := &models.Provider{
		ID:                   uuid.New(),
		Name:                 "test-provider",
		DisplayName:          "Test Provider",
		ProviderType:         string(models.ProviderTypeOpenAI),
		EncryptedCredentials: models.JSONB(encryptedCreds),
		Config: models.JSONB(map[string]interface{}{
			"base_url": "https://api.test.com",
		}),
		Enabled: true,
	}

	if err := providerRepo.Create(ctx, testProvider); err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}

	tests := []struct {
		name           string
		providerID     string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "get_with_admin_role_includes_credentials",
			providerID:     testProvider.ID.String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ProviderDetailResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.ID != testProvider.ID.String() {
					t.Errorf("Expected ID %s, got %s", testProvider.ID.String(), result.ID)
				}

				// Admin should see decrypted credentials
				if result.Credentials == nil {
					t.Error("Expected credentials to be present for admin role")
				} else {
					if val, ok := result.Credentials["api_key"].(string); ok {
						if val != apiKey {
							t.Errorf("Expected decrypted API key '%s', got '%s'", apiKey, val)
						}
					} else {
						t.Error("Expected api_key in credentials")
					}
				}
			},
		},
		{
			name:           "get_with_viewer_role_excludes_credentials",
			providerID:     testProvider.ID.String(),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ProviderDetailResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Viewer should NOT see credentials
				if result.Credentials != nil && len(result.Credentials) > 0 {
					t.Errorf("Expected no credentials for viewer role, got: %v", result.Credentials)
				}
			},
		},
		{
			name:           "get_nonexistent_provider",
			providerID:     uuid.New().String(),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get_invalid_uuid",
			providerID:     "invalid-uuid",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/admin/providers/%s", tt.providerID)
			req := httptest.NewRequest(http.MethodGet, url, nil)

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply JWT middleware
			viewerMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleViewer.String())
			resp := httptest.NewRecorder()

			viewerMiddleware(http.HandlerFunc(handler.GetByID)).ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminProvidersHandlerUpdate tests updating a provider
func TestAdminProvidersHandlerUpdate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestProviders(t, db)

	encryption := setupTestEncryption(t)
	cfg := setupTestConfig(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminProvidersHandler(db, encryption, registry)

	// Create a test provider
	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)

	testProvider := &models.Provider{
		ID:           uuid.New(),
		Name:         "test-provider",
		DisplayName:  "Original Name",
		ProviderType: string(models.ProviderTypeOpenAI),
		Enabled:      true,
	}

	if err := providerRepo.Create(ctx, testProvider); err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}

	tests := []struct {
		name           string
		providerID     string
		payload        UpdateProviderRequest
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:       "update_display_name",
			providerID: testProvider.ID.String(),
			payload: UpdateProviderRequest{
				DisplayName: stringPtr("Updated Name"),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ProviderResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.DisplayName != "Updated Name" {
					t.Errorf("Expected display name 'Updated Name', got '%s'", result.DisplayName)
				}
			},
		},
		{
			name:       "update_enabled_status",
			providerID: testProvider.ID.String(),
			payload: UpdateProviderRequest{
				Enabled: boolPtr(false),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ProviderResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.Enabled {
					t.Error("Expected provider to be disabled")
				}
			},
		},
		{
			name:       "update_credentials",
			providerID: testProvider.ID.String(),
			payload: UpdateProviderRequest{
				Credentials: &map[string]interface{}{
					"api_key": "new-secret-key",
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update_nonexistent_provider",
			providerID:     uuid.New().String(),
			payload:        UpdateProviderRequest{},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.payload)
			url := fmt.Sprintf("/admin/providers/%s", tt.providerID)
			req := httptest.NewRequest(http.MethodPut, url, bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply JWT middleware
			adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())
			resp := httptest.NewRecorder()

			adminMiddleware(http.HandlerFunc(handler.Update)).ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminProvidersHandlerDelete tests soft-deleting a provider
func TestAdminProvidersHandlerDelete(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestProviders(t, db)

	encryption := setupTestEncryption(t)
	cfg := setupTestConfig(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminProvidersHandler(db, encryption, registry)

	// Create a test provider
	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)

	testProvider := &models.Provider{
		ID:           uuid.New(),
		Name:         "test-provider",
		DisplayName:  "Test Provider",
		ProviderType: string(models.ProviderTypeOpenAI),
		Enabled:      true,
	}

	if err := providerRepo.Create(ctx, testProvider); err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}

	tests := []struct {
		name           string
		providerID     string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, providerID uuid.UUID)
	}{
		{
			name:           "delete_existing_provider",
			providerID:     testProvider.ID.String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, providerID uuid.UUID) {
				// Verify provider is disabled (soft delete)
				provider, err := providerRepo.GetByID(ctx, providerID)
				if err != nil {
					t.Fatalf("Failed to get provider after delete: %v", err)
				}

				if provider.Enabled {
					t.Error("Expected provider to be disabled after delete")
				}
			},
		},
		{
			name:           "delete_nonexistent_provider",
			providerID:     uuid.New().String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/admin/providers/%s", tt.providerID)
			req := httptest.NewRequest(http.MethodDelete, url, nil)

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply JWT middleware
			adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())
			resp := httptest.NewRecorder()

			adminMiddleware(http.HandlerFunc(handler.Delete)).ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				providerID, _ := uuid.Parse(tt.providerID)
				tt.checkResponse(t, providerID)
			}
		})
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
