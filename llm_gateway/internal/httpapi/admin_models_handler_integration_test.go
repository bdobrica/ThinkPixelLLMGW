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
	"llm_gateway/internal/utils"
)

// Integration tests for AdminModelsHandler
//
// These tests require a PostgreSQL database to be running.
// Use docker-compose from the root of the repo:
//
//   cd .. && docker-compose up -d postgres
//
// Then run tests:
//   DATABASE_URL="postgres://gateway:password@localhost:5432/gateway?sslmode=disable" go test -v -run TestAdminModels

// cleanupTestModels removes all test models from the database
func cleanupTestModels(t *testing.T, db *storage.DB) {
	t.Helper()

	ctx := context.Background()

	// Delete all models (this will cascade to pricing components)
	// In a real scenario, you might want to be more selective
	_, err := db.Conn().ExecContext(ctx, "DELETE FROM models WHERE model_name LIKE 'test-%'")
	if err != nil {
		t.Logf("Warning: Failed to clean up test models: %v", err)
	}
}

// createTestProvider creates a test provider for model tests
func createTestProvider(t *testing.T, db *storage.DB) *models.Provider {
	t.Helper()

	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)

	provider := &models.Provider{
		ID:           uuid.New(),
		Name:         "test-model-provider",
		DisplayName:  "Test Model Provider",
		ProviderType: string(models.ProviderTypeOpenAI),
		Enabled:      true,
	}

	if err := providerRepo.Create(ctx, provider); err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}

	return provider
}

// cleanupTestProvider removes the test provider
func cleanupTestProvider(t *testing.T, db *storage.DB, providerID uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	providerRepo := storage.NewProviderRepository(db)
	_ = providerRepo.Delete(ctx, providerID)
}

// TestAdminModelsHandlerCreate tests creating a new model
func TestAdminModelsHandlerCreate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminModelsHandler(db, registry)

	// Create test provider
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	tests := []struct {
		name           string
		payload        CreateModelRequest
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name: "successful_create_basic_model",
			payload: CreateModelRequest{
				ModelName:               "test-gpt-4",
				ProviderID:              provider.ID.String(),
				Source:                  "openai",
				Version:                 "2024-11",
				Currency:                "USD",
				SupportsTextInput:       true,
				SupportsTextOutput:      true,
				SupportsFunctionCalling: true,
				SupportsNativeStreaming: true,
				MaxTokens:               128000,
				MaxInputTokens:          128000,
				MaxOutputTokens:         4096,
				PricingComponents: []PricingComponentCreate{
					{
						Code:      "input_text_default",
						Direction: "input",
						Modality:  "text",
						Unit:      "1k_tokens",
						Price:     0.03,
					},
					{
						Code:      "output_text_default",
						Direction: "output",
						Modality:  "text",
						Unit:      "1k_tokens",
						Price:     0.06,
					},
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.ModelName != "test-gpt-4" {
					t.Errorf("Expected model name 'test-gpt-4', got '%s'", result.ModelName)
				}
				if result.ProviderID != provider.ID.String() {
					t.Errorf("Expected provider ID '%s', got '%s'", provider.ID.String(), result.ProviderID)
				}
				if result.Currency != "USD" {
					t.Errorf("Expected currency 'USD', got '%s'", result.Currency)
				}
				if len(result.Features) == 0 {
					t.Error("Expected features to be populated")
				}
			},
		},
		{
			name: "successful_create_with_vision",
			payload: CreateModelRequest{
				ModelName:          "test-gpt-4-vision",
				ProviderID:         provider.ID.String(),
				Source:             "openai",
				Currency:           "USD",
				SupportsTextInput:  true,
				SupportsTextOutput: true,
				SupportsVision:     true,
				SupportsImageInput: true,
				MaxTokens:          128000,
				MaxImagesPerPrompt: 10,
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Check that vision is in features
				hasVision := false
				for _, f := range result.Features {
					if f == "vision" {
						hasVision = true
						break
					}
				}
				if !hasVision {
					t.Error("Expected 'vision' in features list")
				}
			},
		},
		{
			name: "missing_model_name",
			payload: CreateModelRequest{
				ProviderID: provider.ID.String(),
				Source:     "openai",
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing_provider_id",
			payload: CreateModelRequest{
				ModelName: "test-model",
				Source:    "openai",
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid_provider_id",
			payload: CreateModelRequest{
				ModelName:  "test-model",
				ProviderID: "invalid-uuid",
				Source:     "openai",
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "nonexistent_provider",
			payload: CreateModelRequest{
				ModelName:  "test-model",
				ProviderID: uuid.New().String(),
				Source:     "openai",
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "duplicate_model_name",
			payload: CreateModelRequest{
				ModelName:  "test-gpt-4", // Already created above
				ProviderID: provider.ID.String(),
				Source:     "openai",
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/admin/models", bytes.NewBuffer(payload))
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

// TestAdminModelsHandlerList tests listing models
func TestAdminModelsHandlerList(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminModelsHandler(db, registry)

	// Create test provider
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	// Create test models
	ctx := context.Background()

	testModels := []struct {
		name     string
		model    *models.Model
		pricings []models.PricingComponent
	}{
		{
			name: "test-model-1",
			model: &models.Model{
				ID:                uuid.New(),
				ModelName:         "test-model-1",
				ProviderID:        provider.ID.String(),
				Source:            "openai",
				Currency:          "USD",
				SupportsTextInput: true,
				MaxTokens:         100000,
			},
		},
		{
			name: "test-model-2",
			model: &models.Model{
				ID:                uuid.New(),
				ModelName:         "test-model-2",
				ProviderID:        provider.ID.String(),
				Source:            "openai",
				Currency:          "USD",
				SupportsVision:    true,
				SupportsTextInput: true,
				MaxTokens:         50000,
			},
		},
	}

	for _, tm := range testModels {
		// Insert directly using DB connection
		query := `
			INSERT INTO models (
				id, model_name, provider_id, source, currency,
				supports_text_input, supports_vision, max_tokens,
				is_deprecated
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`
		_, err := db.Conn().ExecContext(ctx, query,
			tm.model.ID, tm.model.ModelName, tm.model.ProviderID, tm.model.Source, tm.model.Currency,
			tm.model.SupportsTextInput, tm.model.SupportsVision, tm.model.MaxTokens,
			false,
		)
		if err != nil {
			t.Fatalf("Failed to create test model: %v", err)
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
			name:           "list_all_models",
			queryParams:    "",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var results []ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should have at least our test models
				if len(results) < 2 {
					t.Errorf("Expected at least 2 models, got %d", len(results))
				}
			},
		},
		{
			name:           "filter_by_provider",
			queryParams:    fmt.Sprintf("provider_id=%s", provider.ID.String()),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var results []ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// All results should be from our test provider
				for _, m := range results {
					if m.ProviderID != provider.ID.String() {
						t.Errorf("Expected provider ID '%s', got '%s'", provider.ID.String(), m.ProviderID)
					}
				}
			},
		},
		{
			name:           "search_by_name",
			queryParams:    "search=test-model-1",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var results []ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should find the specific model
				found := false
				for _, m := range results {
					if m.ModelName == "test-model-1" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected to find 'test-model-1' in search results")
				}
			},
		},
		{
			name:           "pagination",
			queryParams:    "limit=1&offset=0",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var results []ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Should have exactly 1 result due to limit
				if len(results) != 1 {
					t.Errorf("Expected 1 model with limit=1, got %d", len(results))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/admin/models"
			if tt.queryParams != "" {
				url += "?" + tt.queryParams
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)

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

// TestAdminModelsHandlerGetByID tests getting a model by ID
func TestAdminModelsHandlerGetByID(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminModelsHandler(db, registry)

	// Create test provider
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	// Create a test model with pricing components
	ctx := context.Background()
	testModel := &models.Model{
		ID:                      uuid.New(),
		ModelName:               "test-detailed-model",
		ProviderID:              provider.ID.String(),
		Source:                  "openai",
		Version:                 "2024-11",
		Currency:                "USD",
		SupportsTextInput:       true,
		SupportsTextOutput:      true,
		SupportsFunctionCalling: true,
		SupportsVision:          true,
		MaxTokens:               128000,
		MaxInputTokens:          120000,
		MaxOutputTokens:         8000,
		AverageLatencyMs:        250.5,
		P95LatencyMs:            500.0,
	}

	// Insert model
	query := `
		INSERT INTO models (
			id, model_name, provider_id, source, version, currency,
			supports_text_input, supports_text_output, supports_function_calling, supports_vision,
			max_tokens, max_input_tokens, max_output_tokens,
			average_latency_ms, p95_latency_ms, is_deprecated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := db.Conn().ExecContext(ctx, query,
		testModel.ID, testModel.ModelName, testModel.ProviderID, testModel.Source, testModel.Version, testModel.Currency,
		testModel.SupportsTextInput, testModel.SupportsTextOutput, testModel.SupportsFunctionCalling, testModel.SupportsVision,
		testModel.MaxTokens, testModel.MaxInputTokens, testModel.MaxOutputTokens,
		testModel.AverageLatencyMs, testModel.P95LatencyMs, false,
	)
	if err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}

	// Insert pricing components
	pricingQuery := `
		INSERT INTO pricing_components (
			id, model_id, code, direction, modality, unit, price
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	pricings := []struct {
		code      string
		direction string
		modality  string
		unit      string
		price     float64
	}{
		{"input_text_default", "input", "text", "1k_tokens", 0.03},
		{"output_text_default", "output", "text", "1k_tokens", 0.06},
	}

	for _, p := range pricings {
		_, err := db.Conn().ExecContext(ctx, pricingQuery,
			uuid.New(), testModel.ID, p.code, p.direction, p.modality, p.unit, p.price,
		)
		if err != nil {
			t.Fatalf("Failed to create pricing component: %v", err)
		}
	}

	tests := []struct {
		name           string
		modelID        string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:           "get_model_details",
			modelID:        testModel.ID.String(),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ModelDetailResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.ID != testModel.ID.String() {
					t.Errorf("Expected ID %s, got %s", testModel.ID.String(), result.ID)
				}
				if result.ModelName != testModel.ModelName {
					t.Errorf("Expected model name '%s', got '%s'", testModel.ModelName, result.ModelName)
				}
				if !result.SupportsFunctionCalling {
					t.Error("Expected function calling support to be true")
				}
				if !result.SupportsVision {
					t.Error("Expected vision support to be true")
				}
				if len(result.PricingComponents) != 2 {
					t.Errorf("Expected 2 pricing components, got %d", len(result.PricingComponents))
				}
				if result.AverageLatencyMs != 250.5 {
					t.Errorf("Expected average latency 250.5ms, got %.2f", result.AverageLatencyMs)
				}
			},
		},
		{
			name:           "get_nonexistent_model",
			modelID:        uuid.New().String(),
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "get_invalid_uuid",
			modelID:        "invalid-uuid",
			roles:          []string{auth.RoleViewer.String()},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/admin/models/%s", tt.modelID)
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

// TestAdminModelsHandlerUpdate tests updating a model
func TestAdminModelsHandlerUpdate(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminModelsHandler(db, registry)

	// Create test provider
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	// Create a test model
	ctx := context.Background()
	testModel := &models.Model{
		ID:         uuid.New(),
		ModelName:  "test-update-model",
		ProviderID: provider.ID.String(),
		Source:     "openai",
		Version:    "v1",
		Currency:   "USD",
	}

	query := `
		INSERT INTO models (
			id, model_name, provider_id, source, version, currency, is_deprecated
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := db.Conn().ExecContext(ctx, query,
		testModel.ID, testModel.ModelName, testModel.ProviderID, testModel.Source,
		testModel.Version, testModel.Currency, false,
	)
	if err != nil {
		t.Fatalf("Failed to create test model: %v", err)
	}

	tests := []struct {
		name           string
		modelID        string
		payload        UpdateModelRequest
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, resp *httptest.ResponseRecorder)
	}{
		{
			name:    "update_version",
			modelID: testModel.ID.String(),
			payload: UpdateModelRequest{
				Version: utils.StringPtr("v2"),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if result.Version != "v2" {
					t.Errorf("Expected version 'v2', got '%s'", result.Version)
				}
			},
		},
		{
			name:    "mark_as_deprecated",
			modelID: testModel.ID.String(),
			payload: UpdateModelRequest{
				IsDeprecated: utils.BoolPtr(true),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				var result ModelResponse
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if !result.IsDeprecated {
					t.Error("Expected model to be marked as deprecated")
				}
			},
		},
		{
			name:    "update_latency_metrics",
			modelID: testModel.ID.String(),
			payload: UpdateModelRequest{
				AverageLatencyMs: utils.Float64Ptr(150.0),
				P95LatencyMs:     utils.Float64Ptr(300.0),
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
		},
		{
			name:    "update_pricing_components",
			modelID: testModel.ID.String(),
			payload: UpdateModelRequest{
				PricingComponents: &[]PricingComponentCreate{
					{
						Code:      "input_text_default",
						Direction: "input",
						Modality:  "text",
						Unit:      "1k_tokens",
						Price:     0.05,
					},
				},
			},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "update_nonexistent_model",
			modelID:        uuid.New().String(),
			payload:        UpdateModelRequest{},
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(tt.payload)
			url := fmt.Sprintf("/admin/models/%s", tt.modelID)
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

// TestAdminModelsHandlerDelete tests deleting a model
func TestAdminModelsHandlerDelete(t *testing.T) {
	skipIfNoDatabase(t)

	db := setupTestDB(t)
	defer db.Close()
	defer cleanupTestModels(t, db)

	cfg := setupTestConfig(t)
	encryption := setupTestEncryption(t)
	registry := setupTestProviderRegistry(t, db, encryption)
	defer registry.Close()

	handler := NewAdminModelsHandler(db, registry)

	// Create test provider
	provider := createTestProvider(t, db)
	defer cleanupTestProvider(t, db, provider.ID)

	// Create test models
	ctx := context.Background()

	modelWithoutAlias := &models.Model{
		ID:         uuid.New(),
		ModelName:  "test-delete-model",
		ProviderID: provider.ID.String(),
		Source:     "openai",
		Currency:   "USD",
	}

	modelWithAlias := &models.Model{
		ID:         uuid.New(),
		ModelName:  "test-model-with-alias",
		ProviderID: provider.ID.String(),
		Source:     "openai",
		Currency:   "USD",
	}

	query := `
		INSERT INTO models (
			id, model_name, provider_id, source, currency, is_deprecated
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	for _, m := range []*models.Model{modelWithoutAlias, modelWithAlias} {
		_, err := db.Conn().ExecContext(ctx, query,
			m.ID, m.ModelName, m.ProviderID, m.Source, m.Currency, false,
		)
		if err != nil {
			t.Fatalf("Failed to create test model: %v", err)
		}
	}

	// Create an alias for the second model
	aliasQuery := `
		INSERT INTO model_aliases (
			id, alias, target_model_id, provider_id, enabled
		) VALUES ($1, $2, $3, $4, $5)
	`
	_, err := db.Conn().ExecContext(ctx, aliasQuery,
		uuid.New(), "test-alias", modelWithAlias.ID, provider.ID, true,
	)
	if err != nil {
		t.Fatalf("Failed to create test alias: %v", err)
	}

	tests := []struct {
		name           string
		modelID        string
		roles          []string
		expectedStatus int
		checkResponse  func(t *testing.T, modelID uuid.UUID)
	}{
		{
			name:           "delete_model_without_aliases",
			modelID:        modelWithoutAlias.ID.String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, modelID uuid.UUID) {
				// Verify model is deleted
				modelRepo := storage.NewModelRepository(db)
				_, err := modelRepo.GetByID(ctx, modelID)
				if err == nil {
					t.Error("Expected model to be deleted")
				}
			},
		},
		{
			name:           "cannot_delete_model_with_aliases",
			modelID:        modelWithAlias.ID.String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, modelID uuid.UUID) {
				// Verify model still exists
				modelRepo := storage.NewModelRepository(db)
				_, err := modelRepo.GetByID(ctx, modelID)
				if err != nil {
					t.Error("Expected model to still exist after failed delete")
				}
			},
		},
		{
			name:           "delete_nonexistent_model",
			modelID:        uuid.New().String(),
			roles:          []string{auth.RoleAdmin.String()},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/admin/models/%s", tt.modelID)
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
				modelID, _ := uuid.Parse(tt.modelID)
				tt.checkResponse(t, modelID)
			}
		})
	}
}
