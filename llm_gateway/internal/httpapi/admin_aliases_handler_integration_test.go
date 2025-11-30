package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
)

// Integration tests for AdminAliasesHandler
//
// These tests require a PostgreSQL database to be running.
// Use docker-compose from the root of the repo:
//
//   docker-compose up -d postgres
//
// Then run tests:
//   DATABASE_URL="postgres://gateway:password@localhost:5432/gateway?sslmode=disable" go test -v -run TestAdminAliases

// cleanupTestAliases removes all test aliases from the database
func cleanupTestAliases(t *testing.T, db *storage.DB) {
	t.Helper()

	ctx := context.Background()

	// Delete all aliases with test names
	_, err := db.Conn().ExecContext(ctx, "DELETE FROM model_aliases WHERE alias LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to clean up test aliases: %v", err)
	}
}

// createTestModel creates a test model for alias tests
func createTestModel(t *testing.T, db *storage.DB, provider *models.Provider, modelName string) *models.Model {
	t.Helper()

	ctx := context.Background()

	testModel := &models.Model{
		ID:                uuid.New(),
		ModelName:         modelName,
		ProviderID:        provider.ID.String(),
		Source:            "openai",
		Version:           "2024-11",
		Currency:          "USD",
		SupportsTextInput: true,
		MaxTokens:         100000,
	}

	query := `
		INSERT INTO models (
			id, model_name, provider_id, source, version, currency,
			supports_text_input, max_tokens, is_deprecated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := db.Conn().ExecContext(ctx, query,
		testModel.ID, testModel.ModelName, testModel.ProviderID, testModel.Source,
		testModel.Version, testModel.Currency, testModel.SupportsTextInput,
		testModel.MaxTokens, false,
	)
	if err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}

	return testModel
}

// TestAdminAliasesHandlerCreate tests creating a new model alias
func TestAdminAliasesHandlerCreate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestAliases(t, db)
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminAliasesHandler(db, registry)

	// Create test provider and model
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	testModel := createTestModel(t, db, provider, "test-model-for-alias")

	tests := []struct {
		name           string
		payload        CreateAliasRequest
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "successful_create_basic_alias",
			payload: CreateAliasRequest{
				AliasName:     "test-gpt-4-turbo",
				TargetModelID: testModel.ID.String(),
				ProviderID:    provider.ID.String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.AliasName != "test-gpt-4-turbo" {
					t.Errorf("Expected alias_name 'test-gpt-4-turbo', got '%s'", alias.AliasName)
				}
				if alias.TargetModelID != testModel.ID.String() {
					t.Errorf("Expected target_model_id '%s', got '%s'", testModel.ID.String(), alias.TargetModelID)
				}
				if alias.ProviderID != provider.ID.String() {
					t.Errorf("Expected provider_id '%s', got '%s'", provider.ID.String(), alias.ProviderID)
				}
				if !alias.Enabled {
					t.Errorf("Expected alias to be enabled by default")
				}
			},
		},
		{
			name: "successful_create_with_tags",
			payload: CreateAliasRequest{
				AliasName:     "test-alias-with-tags",
				TargetModelID: testModel.ID.String(),
				ProviderID:    provider.ID.String(),
				Tags: map[string]string{
					"department":  "engineering",
					"project":     "chatbot",
					"environment": "production",
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(alias.Tags) != 3 {
					t.Errorf("Expected 3 tags, got %d", len(alias.Tags))
				}
				if alias.Tags["department"] != "engineering" {
					t.Errorf("Expected department tag 'engineering', got '%s'", alias.Tags["department"])
				}
				if alias.Tags["project"] != "chatbot" {
					t.Errorf("Expected project tag 'chatbot', got '%s'", alias.Tags["project"])
				}
			},
		},
		{
			name: "successful_create_with_custom_config",
			payload: CreateAliasRequest{
				AliasName:     "test-alias-custom-config",
				TargetModelID: testModel.ID.String(),
				ProviderID:    provider.ID.String(),
				CustomConfig: map[string]interface{}{
					"max_tokens":  4096,
					"temperature": 0.7,
				},
				Enabled: boolPtr(true),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.CustomConfig == nil {
					t.Fatal("Expected custom_config to be set")
				}
				if maxTokens, ok := alias.CustomConfig["max_tokens"].(float64); !ok || maxTokens != 4096 {
					t.Errorf("Expected max_tokens 4096, got %v", alias.CustomConfig["max_tokens"])
				}
			},
		},
		{
			name: "create_disabled_alias",
			payload: CreateAliasRequest{
				AliasName:     "test-disabled-alias",
				TargetModelID: testModel.ID.String(),
				ProviderID:    provider.ID.String(),
				Enabled:       boolPtr(false),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.Enabled {
					t.Errorf("Expected alias to be disabled")
				}
			},
		},
		{
			name: "missing_alias_name",
			payload: CreateAliasRequest{
				TargetModelID: testModel.ID.String(),
				ProviderID:    provider.ID.String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_target_model_id",
			payload: CreateAliasRequest{
				AliasName:  "test-alias",
				ProviderID: provider.ID.String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_provider_id",
			payload: CreateAliasRequest{
				AliasName:     "test-alias",
				TargetModelID: testModel.ID.String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid_target_model_id",
			payload: CreateAliasRequest{
				AliasName:     "test-alias",
				TargetModelID: "invalid-uuid",
				ProviderID:    provider.ID.String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid_provider_id",
			payload: CreateAliasRequest{
				AliasName:     "test-alias",
				TargetModelID: testModel.ID.String(),
				ProviderID:    "invalid-uuid",
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "nonexistent_target_model",
			payload: CreateAliasRequest{
				AliasName:     "test-alias",
				TargetModelID: uuid.New().String(),
				ProviderID:    provider.ID.String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "nonexistent_provider",
			payload: CreateAliasRequest{
				AliasName:     "test-alias",
				TargetModelID: testModel.ID.String(),
				ProviderID:    uuid.New().String(),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/admin/aliases", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply middleware
			adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())
			wrappedHandler := adminMiddleware(http.HandlerFunc(handler.Create))

			resp := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminAliasesHandlerList tests listing model aliases
func TestAdminAliasesHandlerList(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestAliases(t, db)
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminAliasesHandler(db, registry)

	// Create test provider and models
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	model1 := createTestModel(t, db, provider, "test-model-1")
	model2 := createTestModel(t, db, provider, "test-model-2")

	// Create test aliases
	ctx := context.Background()
	aliasRepo := storage.NewModelAliasRepository(db)

	testAliases := []struct {
		alias *models.ModelAlias
		tags  map[string]string
	}{
		{
			alias: &models.ModelAlias{
				ID:            uuid.New(),
				Alias:         "test-alias-1",
				TargetModelID: model1.ID,
				ProviderID:    provider.ID,
				Enabled:       true,
			},
			tags: map[string]string{
				"department": "engineering",
				"project":    "chatbot",
			},
		},
		{
			alias: &models.ModelAlias{
				ID:            uuid.New(),
				Alias:         "test-alias-2",
				TargetModelID: model2.ID,
				ProviderID:    provider.ID,
				Enabled:       true,
			},
			tags: map[string]string{
				"department": "marketing",
				"project":    "analytics",
			},
		},
		{
			alias: &models.ModelAlias{
				ID:            uuid.New(),
				Alias:         "test-alias-3",
				TargetModelID: model1.ID,
				ProviderID:    provider.ID,
				Enabled:       false,
			},
			tags: map[string]string{
				"department": "engineering",
				"project":    "experiment",
			},
		},
	}

	for _, ta := range testAliases {
		if err := aliasRepo.Create(ctx, ta.alias); err != nil {
			t.Fatalf("Failed to create test alias: %v", err)
		}
		for key, value := range ta.tags {
			if err := aliasRepo.SetTag(ctx, ta.alias.ID, key, value); err != nil {
				t.Fatalf("Failed to set tag: %v", err)
			}
		}
	}

	tests := []struct {
		name           string
		queryParams    string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "list_all_aliases",
			queryParams:    "",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should have at least our test aliases
				if result.TotalCount < 3 {
					t.Errorf("Expected at least 3 aliases, got %d", result.TotalCount)
				}
			},
		},
		{
			name:           "filter_by_provider",
			queryParams:    fmt.Sprintf("provider_id=%s", provider.ID.String()),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// All results should be from our test provider
				for _, alias := range result.Aliases {
					if alias.ProviderID != provider.ID.String() {
						t.Errorf("Expected provider_id '%s', got '%s'", provider.ID.String(), alias.ProviderID)
					}
				}
			},
		},
		{
			name:           "search_by_name",
			queryParams:    "search=test-alias-1",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should find the specific alias
				found := false
				for _, alias := range result.Aliases {
					if alias.AliasName == "test-alias-1" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find 'test-alias-1' in search results")
				}
			},
		},
		{
			name:           "filter_by_tags_single",
			queryParams:    "tags=department:engineering",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should find aliases with department:engineering tag
				if result.TotalCount < 2 {
					t.Errorf("Expected at least 2 aliases with department:engineering, got %d", result.TotalCount)
				}

				// Verify all have the correct tag
				for _, alias := range result.Aliases {
					if alias.Tags["department"] != "engineering" {
						t.Errorf("Expected department tag 'engineering', got '%s'", alias.Tags["department"])
					}
				}
			},
		},
		{
			name:           "filter_by_tags_multiple",
			queryParams:    "tags=department:engineering,project:chatbot",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should find only aliases matching both tags
				for _, alias := range result.Aliases {
					if alias.Tags["department"] != "engineering" || alias.Tags["project"] != "chatbot" {
						t.Errorf("Expected alias to have both department:engineering and project:chatbot tags")
					}
				}
			},
		},
		{
			name:           "pagination_page_1",
			queryParams:    "page=1&page_size=2",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should have at most 2 results due to page_size
				if len(result.Aliases) > 2 {
					t.Errorf("Expected at most 2 aliases, got %d", len(result.Aliases))
				}
				if result.Page != 1 {
					t.Errorf("Expected page 1, got %d", result.Page)
				}
				if result.PageSize != 2 {
					t.Errorf("Expected page_size 2, got %d", result.PageSize)
				}
			},
		},
		{
			name:           "combined_filters",
			queryParams:    fmt.Sprintf("provider_id=%s&search=alias-1&tags=department:engineering", provider.ID.String()),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ListAliasesResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should apply all filters
				for _, alias := range result.Aliases {
					if alias.ProviderID != provider.ID.String() {
						t.Errorf("Provider filter not applied correctly")
					}
					if alias.Tags["department"] != "engineering" {
						t.Errorf("Tag filter not applied correctly")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/admin/aliases"
			if tt.queryParams != "" {
				url += "?" + tt.queryParams
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply middleware
			viewerMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleViewer.String())
			wrappedHandler := viewerMiddleware(http.HandlerFunc(handler.List))

			resp := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminAliasesHandlerGetByID tests getting a model alias by ID
func TestAdminAliasesHandlerGetByID(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestAliases(t, db)
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminAliasesHandler(db, registry)

	// Create test provider and model
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	testModel := createTestModel(t, db, provider, "test-model-get")

	// Create a test alias
	ctx := context.Background()
	aliasRepo := storage.NewModelAliasRepository(db)

	testAlias := &models.ModelAlias{
		ID:            uuid.New(),
		Alias:         "test-detailed-alias",
		TargetModelID: testModel.ID,
		ProviderID:    provider.ID,
		CustomConfig:  models.JSONB(map[string]interface{}{"max_tokens": 4096}),
		Enabled:       true,
	}

	if err := aliasRepo.Create(ctx, testAlias); err != nil {
		t.Fatalf("Failed to create test alias: %v", err)
	}

	// Add tags
	tags := map[string]string{
		"department": "engineering",
		"project":    "chatbot",
	}
	for key, value := range tags {
		if err := aliasRepo.SetTag(ctx, testAlias.ID, key, value); err != nil {
			t.Fatalf("Failed to set tag: %v", err)
		}
	}

	tests := []struct {
		name           string
		aliasID        string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "get_alias_details",
			aliasID:        testAlias.ID.String(),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.ID != testAlias.ID.String() {
					t.Errorf("Expected ID '%s', got '%s'", testAlias.ID.String(), alias.ID)
				}
				if alias.AliasName != "test-detailed-alias" {
					t.Errorf("Expected alias_name 'test-detailed-alias', got '%s'", alias.AliasName)
				}
				if alias.TargetModelID != testModel.ID.String() {
					t.Errorf("Expected target_model_id '%s', got '%s'", testModel.ID.String(), alias.TargetModelID)
				}
				if !alias.Enabled {
					t.Errorf("Expected alias to be enabled")
				}

				// Check custom config
				if alias.CustomConfig == nil {
					t.Fatal("Expected custom_config to be set")
				}
				if maxTokens, ok := alias.CustomConfig["max_tokens"].(float64); !ok || maxTokens != 4096 {
					t.Errorf("Expected max_tokens 4096, got %v", alias.CustomConfig["max_tokens"])
				}

				// Check tags
				if len(alias.Tags) != 2 {
					t.Errorf("Expected 2 tags, got %d", len(alias.Tags))
				}
				if alias.Tags["department"] != "engineering" {
					t.Errorf("Expected department tag 'engineering', got '%s'", alias.Tags["department"])
				}
			},
		},
		{
			name:           "get_nonexistent_alias",
			aliasID:        uuid.New().String(),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get_invalid_uuid",
			aliasID:        "invalid-uuid",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/aliases/"+tt.aliasID, nil)

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply middleware
			viewerMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleViewer.String())
			wrappedHandler := viewerMiddleware(http.HandlerFunc(handler.GetByID))

			resp := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminAliasesHandlerUpdate tests updating a model alias
func TestAdminAliasesHandlerUpdate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestAliases(t, db)
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminAliasesHandler(db, registry)

	// Create test provider and models
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	model1 := createTestModel(t, db, provider, "test-model-update-1")
	model2 := createTestModel(t, db, provider, "test-model-update-2")

	// Create a test alias
	ctx := context.Background()
	aliasRepo := storage.NewModelAliasRepository(db)

	testAlias := &models.ModelAlias{
		ID:            uuid.New(),
		Alias:         "test-update-alias",
		TargetModelID: model1.ID,
		ProviderID:    provider.ID,
		Enabled:       true,
	}

	if err := aliasRepo.Create(ctx, testAlias); err != nil {
		t.Fatalf("Failed to create test alias: %v", err)
	}

	// Add initial tags
	if err := aliasRepo.SetTag(ctx, testAlias.ID, "department", "engineering"); err != nil {
		t.Fatalf("Failed to set tag: %v", err)
	}

	tests := []struct {
		name           string
		aliasID        string
		payload        UpdateAliasRequest
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:    "update_alias_name",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				AliasName: stringPtr("test-updated-alias"),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.AliasName != "test-updated-alias" {
					t.Errorf("Expected alias_name 'test-updated-alias', got '%s'", alias.AliasName)
				}
			},
		},
		{
			name:    "update_target_model",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				TargetModelID: stringPtr(model2.ID.String()),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.TargetModelID != model2.ID.String() {
					t.Errorf("Expected target_model_id '%s', got '%s'", model2.ID.String(), alias.TargetModelID)
				}
			},
		},
		{
			name:    "update_enabled_status",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				Enabled: boolPtr(false),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.Enabled {
					t.Errorf("Expected alias to be disabled")
				}
			},
		},
		{
			name:    "update_custom_config",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				CustomConfig: map[string]interface{}{
					"max_tokens":  8192,
					"temperature": 0.5,
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if alias.CustomConfig == nil {
					t.Fatal("Expected custom_config to be set")
				}
				if maxTokens, ok := alias.CustomConfig["max_tokens"].(float64); !ok || maxTokens != 8192 {
					t.Errorf("Expected max_tokens 8192, got %v", alias.CustomConfig["max_tokens"])
				}
			},
		},
		{
			name:    "update_tags",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				Tags: map[string]string{
					"department": "marketing",
					"project":    "analytics",
					"region":     "us-east",
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var alias AliasResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &alias); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(alias.Tags) != 3 {
					t.Errorf("Expected 3 tags, got %d", len(alias.Tags))
				}
				if alias.Tags["department"] != "marketing" {
					t.Errorf("Expected department tag 'marketing', got '%s'", alias.Tags["department"])
				}
				if alias.Tags["project"] != "analytics" {
					t.Errorf("Expected project tag 'analytics', got '%s'", alias.Tags["project"])
				}
			},
		},
		{
			name:    "update_multiple_fields",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				AliasName: stringPtr("test-multi-update"),
				Enabled:   boolPtr(true),
				Tags: map[string]string{
					"environment": "production",
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update_nonexistent_alias",
			aliasID:        uuid.New().String(),
			payload:        UpdateAliasRequest{},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:    "update_with_invalid_target_model",
			aliasID: testAlias.ID.String(),
			payload: UpdateAliasRequest{
				TargetModelID: stringPtr(uuid.New().String()),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPut, "/admin/aliases/"+tt.aliasID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply middleware
			adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())
			wrappedHandler := adminMiddleware(http.HandlerFunc(handler.Update))

			resp := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

// TestAdminAliasesHandlerDelete tests deleting a model alias
func TestAdminAliasesHandlerDelete(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestAliases(t, db)
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminAliasesHandler(db, registry)

	// Create test provider and model
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	testModel := createTestModel(t, db, provider, "test-model-delete")

	// Create test aliases
	ctx := context.Background()
	aliasRepo := storage.NewModelAliasRepository(db)

	aliasToDelete := &models.ModelAlias{
		ID:            uuid.New(),
		Alias:         "test-delete-alias",
		TargetModelID: testModel.ID,
		ProviderID:    provider.ID,
		Enabled:       true,
	}

	if err := aliasRepo.Create(ctx, aliasToDelete); err != nil {
		t.Fatalf("Failed to create test alias: %v", err)
	}

	tests := []struct {
		name           string
		aliasID        string
		roles          []string
		expectedStatus int
		checkDeleted   bool
	}{
		{
			name:           "successful_delete",
			aliasID:        aliasToDelete.ID.String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNoContent,
			checkDeleted:   true,
		},
		{
			name:           "delete_nonexistent_alias",
			aliasID:        uuid.New().String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
			checkDeleted:   false,
		},
		{
			name:           "delete_invalid_uuid",
			aliasID:        "invalid-uuid",
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
			checkDeleted:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/admin/aliases/"+tt.aliasID, nil)

			// Add JWT token
			token := generateAdminJWT(t, cfg, tt.roles...)
			req.Header.Set("Authorization", "Bearer "+token)

			// Apply middleware
			adminMiddleware := middleware.AdminJWTMiddleware(cfg, auth.RoleAdmin.String())
			wrappedHandler := adminMiddleware(http.HandlerFunc(handler.Delete))

			resp := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(resp, req)

			if resp.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.Code, resp.Body.String())
			}

			// Verify deletion
			if tt.checkDeleted {
				aliasID, _ := uuid.Parse(tt.aliasID)
				_, err := aliasRepo.GetByID(ctx, aliasID)
				if err != storage.ErrModelAliasNotFound {
					t.Errorf("Expected alias to be deleted, but it still exists")
				}
			}
		})
	}
}
