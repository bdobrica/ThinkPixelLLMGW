package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

// AdminProvidersHandler handles provider management endpoints
type AdminProvidersHandler struct {
	db         *storage.DB
	encryption *storage.Encryption
	registry   providers.Registry
}

// NewAdminProvidersHandler creates a new admin providers handler
func NewAdminProvidersHandler(db *storage.DB, encryption *storage.Encryption, registry providers.Registry) *AdminProvidersHandler {
	return &AdminProvidersHandler{
		db:         db,
		encryption: encryption,
		registry:   registry,
	}
}

// CreateProviderRequest represents the request to create a new provider
type CreateProviderRequest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Type        string                 `json:"type"`
	Credentials map[string]interface{} `json:"credentials"`
	Config      map[string]interface{} `json:"config"`
	Enabled     bool                   `json:"enabled"`
}

// UpdateProviderRequest represents the request to update a provider
type UpdateProviderRequest struct {
	DisplayName *string                 `json:"display_name,omitempty"`
	Credentials *map[string]interface{} `json:"credentials,omitempty"`
	Config      *map[string]interface{} `json:"config,omitempty"`
	Enabled     *bool                   `json:"enabled,omitempty"`
}

// ProviderResponse represents a provider response (without credentials)
type ProviderResponse struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Type        string                 `json:"type"`
	Config      map[string]interface{} `json:"config"`
	Enabled     bool                   `json:"enabled"`
	ModelCount  int                    `json:"model_count"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// ProviderDetailResponse represents a detailed provider response (with credentials for admins)
type ProviderDetailResponse struct {
	ProviderResponse
	Credentials map[string]interface{} `json:"credentials,omitempty"`
	Models      []ModelInfo            `json:"models"`
}

// ModelInfo represents basic model information
type ModelInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	Deprecated bool   `json:"deprecated"`
}

// Create handles POST /admin/providers - Create new provider
func (h *AdminProvidersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if req.Name == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Provider name is required")
		return
	}
	if req.Type == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Provider type is required")
		return
	}

	// Validate provider type
	validTypes := map[string]bool{
		string(models.ProviderTypeOpenAI):   true,
		string(models.ProviderTypeVertexAI): true,
		string(models.ProviderTypeBedrock):  true,
	}
	if !validTypes[req.Type] {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider type")
		return
	}

	// Encrypt credentials
	encryptedCreds := make(map[string]interface{})
	for key, value := range req.Credentials {
		strValue, ok := value.(string)
		if !ok {
			utils.RespondWithError(w, http.StatusBadRequest, "All credential values must be strings")
			return
		}
		encrypted, err := h.encryption.Encrypt([]byte(strValue))
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to encrypt credentials")
			return
		}
		encryptedCreds[key] = encrypted
	}

	// Create provider
	provider := &models.Provider{
		ID:                   uuid.New(),
		Name:                 req.Name,
		DisplayName:          req.DisplayName,
		ProviderType:         req.Type,
		EncryptedCredentials: models.JSONB(encryptedCreds),
		Config:               models.JSONB(req.Config),
		Enabled:              req.Enabled,
	}

	providerRepo := storage.NewProviderRepository(h.db)
	if err := providerRepo.Create(r.Context(), provider); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			utils.RespondWithError(w, http.StatusConflict, "Provider with this name already exists")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create provider")
		return
	}

	// Trigger registry reload
	if err := h.registry.Reload(r.Context()); err != nil {
		// Log error but don't fail the request
		// The provider is created, reload will happen on next interval
	}

	response := &ProviderResponse{
		ID:          provider.ID.String(),
		Name:        provider.Name,
		DisplayName: provider.DisplayName,
		Type:        provider.ProviderType,
		Config:      req.Config,
		Enabled:     provider.Enabled,
		ModelCount:  0,
		CreatedAt:   provider.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   provider.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	utils.RespondWithJSON(w, http.StatusCreated, response)
}

// List handles GET /admin/providers - List all providers
func (h *AdminProvidersHandler) List(w http.ResponseWriter, r *http.Request) {
	providerRepo := storage.NewProviderRepository(h.db)
	providers, err := providerRepo.List(r.Context())
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list providers")
		return
	}

	// Get model counts for each provider
	modelRepo := storage.NewModelRepository(h.db)

	responses := make([]ProviderResponse, 0, len(providers))
	for _, p := range providers {
		models, _ := modelRepo.GetByProvider(r.Context(), p.ID.String())
		modelCount := len(models)

		var config map[string]interface{}
		if p.Config != nil {
			config = p.Config
		} else {
			config = make(map[string]interface{})
		}

		responses = append(responses, ProviderResponse{
			ID:          p.ID.String(),
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Type:        p.ProviderType,
			Config:      config,
			Enabled:     p.Enabled,
			ModelCount:  modelCount,
			CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	utils.RespondWithJSON(w, http.StatusOK, responses)
}

// GetByID handles GET /admin/providers/:id - Get provider details
func (h *AdminProvidersHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	// Extract provider ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}
	providerIDStr := pathParts[2]

	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID format")
		return
	}

	providerRepo := storage.NewProviderRepository(h.db)
	provider, err := providerRepo.GetByID(r.Context(), providerID)
	if err != nil {
		if err == storage.ErrProviderNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "Provider not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get provider")
		return
	}

	// Get associated models
	modelRepo := storage.NewModelRepository(h.db)
	models, _ := modelRepo.GetByProvider(r.Context(), providerID.String())

	modelInfos := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		modelInfos = append(modelInfos, ModelInfo{
			ID:         m.ID.String(),
			Name:       m.ModelName,
			Source:     m.Source,
			Deprecated: m.IsDeprecated,
		})
	}

	var config map[string]interface{}
	if provider.Config != nil {
		config = provider.Config
	} else {
		config = make(map[string]interface{})
	}

	response := &ProviderDetailResponse{
		ProviderResponse: ProviderResponse{
			ID:          provider.ID.String(),
			Name:        provider.Name,
			DisplayName: provider.DisplayName,
			Type:        provider.ProviderType,
			Config:      config,
			Enabled:     provider.Enabled,
			ModelCount:  len(modelInfos),
			CreatedAt:   provider.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   provider.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
		Models: modelInfos,
	}

	// Only include decrypted credentials for admin role
	if middleware.HasRole(r.Context(), auth.RoleAdmin.String()) {
		decryptedCreds := make(map[string]interface{})
		if provider.EncryptedCredentials != nil {
			for key, value := range provider.EncryptedCredentials {
				strValue, ok := value.(string)
				if !ok {
					continue
				}
				decrypted, err := h.encryption.Decrypt(strValue)
				if err != nil {
					continue
				}
				decryptedCreds[key] = string(decrypted)
			}
		}
		response.Credentials = decryptedCreds
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// Update handles PUT /admin/providers/:id - Update provider
func (h *AdminProvidersHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Extract provider ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}
	providerIDStr := pathParts[2]

	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID format")
		return
	}

	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	providerRepo := storage.NewProviderRepository(h.db)
	provider, err := providerRepo.GetByID(r.Context(), providerID)
	if err != nil {
		if err == storage.ErrProviderNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "Provider not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get provider")
		return
	}

	// Update fields if provided
	if req.DisplayName != nil {
		provider.DisplayName = *req.DisplayName
	}

	if req.Config != nil {
		provider.Config = models.JSONB(*req.Config)
	}

	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}

	if req.Credentials != nil {
		// Re-encrypt credentials
		encryptedCreds := make(map[string]interface{})
		for key, value := range *req.Credentials {
			strValue, ok := value.(string)
			if !ok {
				utils.RespondWithError(w, http.StatusBadRequest, "All credential values must be strings")
				return
			}
			encrypted, err := h.encryption.Encrypt([]byte(strValue))
			if err != nil {
				utils.RespondWithError(w, http.StatusInternalServerError, "Failed to encrypt credentials")
				return
			}
			encryptedCreds[key] = encrypted
		}
		provider.EncryptedCredentials = models.JSONB(encryptedCreds)
	}

	if err := providerRepo.Update(r.Context(), provider); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update provider")
		return
	}

	// Trigger registry reload
	if err := h.registry.Reload(r.Context()); err != nil {
		// Log error but don't fail the request
	}

	var config map[string]interface{}
	if provider.Config != nil {
		config = provider.Config
	} else {
		config = make(map[string]interface{})
	}

	response := &ProviderResponse{
		ID:          provider.ID.String(),
		Name:        provider.Name,
		DisplayName: provider.DisplayName,
		Type:        provider.ProviderType,
		Config:      config,
		Enabled:     provider.Enabled,
		CreatedAt:   provider.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   provider.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// Delete handles DELETE /admin/providers/:id - Soft delete provider (set enabled=false)
func (h *AdminProvidersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Extract provider ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}
	providerIDStr := pathParts[2]

	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID format")
		return
	}

	providerRepo := storage.NewProviderRepository(h.db)
	provider, err := providerRepo.GetByID(r.Context(), providerID)
	if err != nil {
		if err == storage.ErrProviderNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "Provider not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get provider")
		return
	}

	// Soft delete: set enabled to false
	provider.Enabled = false
	if err := providerRepo.Update(r.Context(), provider); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to disable provider")
		return
	}

	// Trigger registry reload
	if err := h.registry.Reload(r.Context()); err != nil {
		// Log error but don't fail the request
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Provider disabled successfully",
	})
}
