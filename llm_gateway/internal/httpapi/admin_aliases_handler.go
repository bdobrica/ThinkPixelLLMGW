package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
)

// AdminAliasesHandler handles model alias management endpoints
type AdminAliasesHandler struct {
	db       *storage.DB
	registry providers.Registry
}

// NewAdminAliasesHandler creates a new admin aliases handler
func NewAdminAliasesHandler(db *storage.DB, registry providers.Registry) *AdminAliasesHandler {
	return &AdminAliasesHandler{
		db:       db,
		registry: registry,
	}
}

// CreateAliasRequest represents the request to create a new model alias
type CreateAliasRequest struct {
	AliasName     string                 `json:"alias_name"`
	TargetModelID string                 `json:"target_model_id"`
	ProviderID    string                 `json:"provider_id"`
	CustomConfig  map[string]interface{} `json:"custom_config,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"` // Pointer to allow explicit false
	Tags          map[string]string      `json:"tags,omitempty"`
}

// UpdateAliasRequest represents the request to update a model alias
type UpdateAliasRequest struct {
	AliasName     *string                `json:"alias_name,omitempty"`
	TargetModelID *string                `json:"target_model_id,omitempty"`
	ProviderID    *string                `json:"provider_id,omitempty"`
	CustomConfig  map[string]interface{} `json:"custom_config,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
	Tags          map[string]string      `json:"tags,omitempty"`
}

// AliasResponse represents the response for a model alias
type AliasResponse struct {
	ID            string                 `json:"id"`
	AliasName     string                 `json:"alias_name"`
	TargetModelID string                 `json:"target_model_id"`
	ProviderID    string                 `json:"provider_id"`
	CustomConfig  map[string]interface{} `json:"custom_config,omitempty"`
	Enabled       bool                   `json:"enabled"`
	Tags          map[string]string      `json:"tags,omitempty"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// ListAliasesResponse represents the paginated response for listing aliases
type ListAliasesResponse struct {
	Aliases    []AliasResponse `json:"aliases"`
	TotalCount int             `json:"total_count"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
}

// Create handles POST /admin/aliases - Create a new model alias
func (h *AdminAliasesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.AliasName == "" {
		http.Error(w, "alias_name is required", http.StatusBadRequest)
		return
	}
	if req.TargetModelID == "" {
		http.Error(w, "target_model_id is required", http.StatusBadRequest)
		return
	}
	if req.ProviderID == "" {
		http.Error(w, "provider_id is required", http.StatusBadRequest)
		return
	}

	// Parse UUIDs
	targetModelID, err := uuid.Parse(req.TargetModelID)
	if err != nil {
		http.Error(w, "Invalid target_model_id format", http.StatusBadRequest)
		return
	}

	providerID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		http.Error(w, "Invalid provider_id format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Validate that the target model exists
	modelRepo := storage.NewModelRepository(h.db)
	targetModel, err := modelRepo.GetByID(ctx, targetModelID)
	if err != nil {
		if err == storage.ErrModelNotFound {
			http.Error(w, "Target model not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to validate target model: %v", err), http.StatusInternalServerError)
		return
	}

	// Validate that the provider exists and is enabled
	providerRepo := storage.NewProviderRepository(h.db)
	provider, err := providerRepo.GetByID(ctx, providerID)
	if err != nil {
		if err == storage.ErrProviderNotFound {
			http.Error(w, "Provider not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to validate provider: %v", err), http.StatusInternalServerError)
		return
	}

	if !provider.Enabled {
		http.Error(w, "Provider is not enabled", http.StatusBadRequest)
		return
	}

	// Validate that the model belongs to the provider or is compatible
	// ProviderID in Model is stored as string, so convert UUID to string for comparison
	if targetModel.ProviderID != providerID.String() {
		http.Error(w, "Target model does not belong to the specified provider", http.StatusBadRequest)
		return
	}

	// Create the alias
	alias := &models.ModelAlias{
		ID:            uuid.New(),
		Alias:         req.AliasName,
		TargetModelID: targetModelID,
		ProviderID:    providerID,
		Enabled:       true, // Default to enabled
	}

	// Set enabled if explicitly provided
	if req.Enabled != nil {
		alias.Enabled = *req.Enabled
	}

	// Set custom config if provided
	if req.CustomConfig != nil {
		alias.CustomConfig = models.JSONB(req.CustomConfig)
	}

	// Create the alias in the database
	aliasRepo := storage.NewModelAliasRepository(h.db)
	if err := aliasRepo.Create(ctx, alias); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create alias: %v", err), http.StatusInternalServerError)
		return
	}

	// Set tags if provided
	if len(req.Tags) > 0 {
		for key, value := range req.Tags {
			if err := aliasRepo.SetTag(ctx, alias.ID, key, value); err != nil {
				// Log the error but don't fail the creation
				http.Error(w, fmt.Sprintf("Failed to set tag: %v", err), http.StatusInternalServerError)
				return
			}
		}
		alias.Tags = req.Tags
	}

	// Return the created alias
	response := h.toAliasResponse(alias)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// List handles GET /admin/aliases - List all aliases with filtering and pagination
func (h *AdminAliasesHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	query := r.URL.Query()

	// Parse pagination parameters
	page := 1
	if p := query.Get("page"); p != "" {
		if parsed, parseErr := strconv.Atoi(p); parseErr == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20 // default page size
	if ps := query.Get("page_size"); ps != "" {
		if parsed, parseErr := strconv.Atoi(ps); parseErr == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	// Get filter parameters
	providerIDStr := query.Get("provider_id")
	search := query.Get("search")
	tagsFilter := query.Get("tags") // Format: "key1:value1,key2:value2"

	// Parse tags filter
	var tagsMap map[string]string
	if tagsFilter != "" {
		tagsMap = make(map[string]string)
		tagPairs := strings.Split(tagsFilter, ",")
		for _, pair := range tagPairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				tagsMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	// Create filters
	filters := storage.AliasListFilters{
		ProviderID: providerIDStr,
		Search:     search,
		Tags:       tagsMap,
		Page:       page,
		PageSize:   pageSize,
	}

	aliasRepo := storage.NewModelAliasRepository(h.db)
	result, err := aliasRepo.ListWithFilters(ctx, filters)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list aliases: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	responses := make([]AliasResponse, len(result.Aliases))
	for i, alias := range result.Aliases {
		responses[i] = h.toAliasResponse(alias)
	}

	// Return paginated response
	response := ListAliasesResponse{
		Aliases:    responses,
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetByID handles GET /admin/aliases/:id - Get alias details
func (h *AdminAliasesHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/aliases/")
	if idStr == "" || idStr == r.URL.Path {
		http.Error(w, "Alias ID is required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid alias ID format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	aliasRepo := storage.NewModelAliasRepository(h.db)

	alias, err := aliasRepo.GetByID(ctx, id)
	if err != nil {
		if err == storage.ErrModelAliasNotFound {
			http.Error(w, "Alias not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get alias: %v", err), http.StatusInternalServerError)
		return
	}

	response := h.toAliasResponse(alias)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Update handles PUT /admin/aliases/:id - Update an alias
func (h *AdminAliasesHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/aliases/")
	if idStr == "" || idStr == r.URL.Path {
		http.Error(w, "Alias ID is required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid alias ID format", http.StatusBadRequest)
		return
	}

	var req UpdateAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	aliasRepo := storage.NewModelAliasRepository(h.db)

	// Get existing alias
	alias, err := aliasRepo.GetByID(ctx, id)
	if err != nil {
		if err == storage.ErrModelAliasNotFound {
			http.Error(w, "Alias not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get alias: %v", err), http.StatusInternalServerError)
		return
	}

	// Update fields if provided
	if req.AliasName != nil {
		alias.Alias = *req.AliasName
	}

	if req.TargetModelID != nil {
		targetModelID, err := uuid.Parse(*req.TargetModelID)
		if err != nil {
			http.Error(w, "Invalid target_model_id format", http.StatusBadRequest)
			return
		}

		// Validate that the target model exists
		modelRepo := storage.NewModelRepository(h.db)
		targetModel, err := modelRepo.GetByID(ctx, targetModelID)
		if err != nil {
			if err == storage.ErrModelNotFound {
				http.Error(w, "Target model not found", http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("Failed to validate target model: %v", err), http.StatusInternalServerError)
			return
		}

		// Ensure model belongs to the same provider (or update provider too)
		// ProviderID in Model is stored as string, so convert UUID to string for comparison
		if req.ProviderID == nil && targetModel.ProviderID != alias.ProviderID.String() {
			http.Error(w, "Target model does not belong to current provider. Please specify provider_id", http.StatusBadRequest)
			return
		}

		alias.TargetModelID = targetModelID
	}

	if req.ProviderID != nil {
		providerID, err := uuid.Parse(*req.ProviderID)
		if err != nil {
			http.Error(w, "Invalid provider_id format", http.StatusBadRequest)
			return
		}

		// Validate that the provider exists and is enabled
		providerRepo := storage.NewProviderRepository(h.db)
		provider, err := providerRepo.GetByID(ctx, providerID)
		if err != nil {
			if err == storage.ErrProviderNotFound {
				http.Error(w, "Provider not found", http.StatusNotFound)
				return
			}
			http.Error(w, fmt.Sprintf("Failed to validate provider: %v", err), http.StatusInternalServerError)
			return
		}

		if !provider.Enabled {
			http.Error(w, "Provider is not enabled", http.StatusBadRequest)
			return
		}

		alias.ProviderID = providerID
	}

	if req.CustomConfig != nil {
		alias.CustomConfig = models.JSONB(req.CustomConfig)
	}

	if req.Enabled != nil {
		alias.Enabled = *req.Enabled
	}

	// Update the alias
	if err := aliasRepo.Update(ctx, alias); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update alias: %v", err), http.StatusInternalServerError)
		return
	}

	// Update tags if provided
	if req.Tags != nil {
		// Clear all existing tags and set new ones
		// Note: We could implement a more sophisticated merge strategy if needed
		for key, value := range req.Tags {
			if err := aliasRepo.SetTag(ctx, alias.ID, key, value); err != nil {
				http.Error(w, fmt.Sprintf("Failed to set tag: %v", err), http.StatusInternalServerError)
				return
			}
		}
		alias.Tags = req.Tags
	}

	// Reload the provider registry to pick up alias changes
	// Note: This is async and errors are logged internally
	go h.registry.Reload(ctx)

	// Return updated alias
	response := h.toAliasResponse(alias)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles DELETE /admin/aliases/:id - Delete an alias
func (h *AdminAliasesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/aliases/")
	if idStr == "" || idStr == r.URL.Path {
		http.Error(w, "Alias ID is required", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid alias ID format", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	aliasRepo := storage.NewModelAliasRepository(h.db)

	// Check if the alias exists
	_, err = aliasRepo.GetByID(ctx, id)
	if err != nil {
		if err == storage.ErrModelAliasNotFound {
			http.Error(w, "Alias not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get alias: %v", err), http.StatusInternalServerError)
		return
	}

	// TODO: Check for dependent aliases before deletion
	// This would require a query to find aliases that reference this alias
	// For now, we'll skip this check

	// Delete the alias
	if err := aliasRepo.Delete(ctx, id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete alias: %v", err), http.StatusInternalServerError)
		return
	}

	// Reload the provider registry to pick up alias changes
	// Note: This is async and errors are logged internally
	go h.registry.Reload(ctx)

	w.WriteHeader(http.StatusNoContent)
}

// toAliasResponse converts a models.ModelAlias to AliasResponse
func (h *AdminAliasesHandler) toAliasResponse(alias *models.ModelAlias) AliasResponse {
	response := AliasResponse{
		ID:            alias.ID.String(),
		AliasName:     alias.Alias,
		TargetModelID: alias.TargetModelID.String(),
		ProviderID:    alias.ProviderID.String(),
		Enabled:       alias.Enabled,
		CreatedAt:     alias.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     alias.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if alias.CustomConfig != nil {
		response.CustomConfig = map[string]interface{}(alias.CustomConfig)
	}

	if alias.Tags != nil {
		response.Tags = alias.Tags
	}

	return response
}
