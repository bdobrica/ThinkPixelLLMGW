package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

// AdminAPIKeysHandler handles API key management endpoints
type AdminAPIKeysHandler struct {
	db *storage.DB
}

// NewAdminAPIKeysHandler creates a new admin API keys handler
func NewAdminAPIKeysHandler(db *storage.DB) *AdminAPIKeysHandler {
	return &AdminAPIKeysHandler{
		db: db,
	}
}

// CreateAPIKeyRequest represents the request to create a new API key
type CreateAPIKeyRequest struct {
	Name               string            `json:"name"`
	AllowedModels      []string          `json:"allowed_models,omitempty"`
	RateLimitPerMinute int               `json:"rate_limit_per_minute"`
	MonthlyBudgetUSD   *float64          `json:"monthly_budget_usd,omitempty"`
	Enabled            *bool             `json:"enabled,omitempty"`
	ExpiresAt          *string           `json:"expires_at,omitempty"` // RFC3339 format
	Tags               map[string]string `json:"tags,omitempty"`
}

// UpdateAPIKeyRequest represents the request to update an API key
type UpdateAPIKeyRequest struct {
	Name               *string           `json:"name,omitempty"`
	AllowedModels      []string          `json:"allowed_models,omitempty"`
	RateLimitPerMinute *int              `json:"rate_limit_per_minute,omitempty"`
	MonthlyBudgetUSD   *float64          `json:"monthly_budget_usd,omitempty"`
	Enabled            *bool             `json:"enabled,omitempty"`
	ExpiresAt          *string           `json:"expires_at,omitempty"` // RFC3339 format, null to remove
	Tags               map[string]string `json:"tags,omitempty"`
}

// APIKeyResponse represents an API key response (without plaintext key or hash)
type APIKeyResponse struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	AllowedModels      []string          `json:"allowed_models"`
	RateLimitPerMinute int               `json:"rate_limit_per_minute"`
	MonthlyBudgetUSD   *float64          `json:"monthly_budget_usd,omitempty"`
	Enabled            bool              `json:"enabled"`
	ExpiresAt          *string           `json:"expires_at,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
}

// APIKeyDetailResponse represents a detailed API key response with usage stats
type APIKeyDetailResponse struct {
	APIKeyResponse
	UsageStats UsageStats `json:"usage_stats"`
}

// APIKeyCreatedResponse represents the response when creating a new API key
// This is the ONLY time the plaintext key is returned
type APIKeyCreatedResponse struct {
	APIKeyResponse
	Key string `json:"key"` // Plaintext key - only returned once
}

// UsageStats represents usage statistics for an API key
type UsageStats struct {
	TotalRequests   int     `json:"total_requests"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	CurrentMonthUSD float64 `json:"current_month_usd"`
	LastUsedAt      *string `json:"last_used_at,omitempty"`
}

// generateAPIKey generates a cryptographically secure random API key
// Format: sk-<32 random hex characters> (total length: 35 characters)
func generateAPIKey() (string, error) {
	bytes := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(bytes), nil
}

// hashAPIKey hashes an API key using SHA-256
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// Create handles POST /admin/keys - Create new API key
func (h *AdminAPIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if req.Name == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Set defaults
	if req.RateLimitPerMinute == 0 {
		req.RateLimitPerMinute = 60 // Default: 60 requests per minute
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Parse expiration date if provided
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsedTime, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid expires_at format (use RFC3339)")
			return
		}
		expiresAt = &parsedTime
	}

	// Generate the API key
	plaintextKey, err := generateAPIKey()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	// Hash the key for storage
	keyHash := hashAPIKey(plaintextKey)

	// Create the API key model
	apiKey := &models.APIKey{
		ID:                 uuid.New(),
		Name:               req.Name,
		KeyHash:            keyHash,
		AllowedModels:      pq.StringArray(req.AllowedModels),
		RateLimitPerMinute: req.RateLimitPerMinute,
		MonthlyBudgetUSD:   req.MonthlyBudgetUSD,
		Enabled:            enabled,
		ExpiresAt:          expiresAt,
	}

	// Create in database
	apiKeyRepo := storage.NewAPIKeyRepository(h.db)
	if err := apiKeyRepo.Create(r.Context(), apiKey); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			utils.RespondWithError(w, http.StatusConflict, "API key already exists")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create API key")
		return
	}

	// Set tags if provided
	if len(req.Tags) > 0 {
		for key, value := range req.Tags {
			if err := apiKeyRepo.SetTag(r.Context(), apiKey.ID, key, value); err != nil {
				// Log error but don't fail the request
				continue
			}
		}
		apiKey.Tags = req.Tags
	}

	// Return response with plaintext key (ONLY TIME IT'S VISIBLE)
	response := &APIKeyCreatedResponse{
		APIKeyResponse: h.toAPIKeyResponse(apiKey),
		Key:            plaintextKey,
	}

	utils.RespondWithJSON(w, http.StatusCreated, response)
}

// List handles GET /admin/keys - List all API keys
func (h *AdminAPIKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	// Pagination parameters
	page := 1
	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 20 // default page size
	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize

	// Get API keys from database
	apiKeyRepo := storage.NewAPIKeyRepository(h.db)
	keys, err := apiKeyRepo.List(r.Context(), pageSize, offset)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}

	// Convert to response format
	responses := make([]APIKeyResponse, 0, len(keys))
	for _, key := range keys {
		responses = append(responses, h.toAPIKeyResponse(key))
	}

	// Get total count (simplified - in production, you'd have a separate count query)
	totalCount := len(responses)
	if len(responses) == pageSize {
		// If we got a full page, there might be more
		totalCount = page * pageSize // Approximate
	}

	// Return paginated response
	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"items":       responses,
		"total_count": totalCount,
		"page":        page,
		"page_size":   pageSize,
	})
}

// GetByID handles GET /admin/keys/:id - Get API key details with usage stats
func (h *AdminAPIKeysHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	// Extract key ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}
	keyIDStr := pathParts[2]

	// Handle regenerate endpoint
	if len(pathParts) == 4 && pathParts[3] == "regenerate" {
		h.Regenerate(w, r)
		return
	}

	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID format")
		return
	}

	apiKeyRepo := storage.NewAPIKeyRepository(h.db)
	apiKey, err := apiKeyRepo.GetByID(r.Context(), keyID)
	if err != nil {
		if err == storage.ErrAPIKeyNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "API key not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get API key")
		return
	}

	// Get usage statistics
	usageStats := h.getUsageStats(r.Context(), keyID)

	response := &APIKeyDetailResponse{
		APIKeyResponse: h.toAPIKeyResponse(apiKey),
		UsageStats:     usageStats,
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// Update handles PUT /admin/keys/:id - Update API key
func (h *AdminAPIKeysHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Extract key ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}
	keyIDStr := pathParts[2]

	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID format")
		return
	}

	var req UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	apiKeyRepo := storage.NewAPIKeyRepository(h.db)
	apiKey, err := apiKeyRepo.GetByID(r.Context(), keyID)
	if err != nil {
		if err == storage.ErrAPIKeyNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "API key not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get API key")
		return
	}

	// Update fields if provided
	if req.Name != nil {
		apiKey.Name = *req.Name
	}

	if req.AllowedModels != nil {
		apiKey.AllowedModels = pq.StringArray(req.AllowedModels)
	}

	if req.RateLimitPerMinute != nil {
		apiKey.RateLimitPerMinute = *req.RateLimitPerMinute
	}

	if req.MonthlyBudgetUSD != nil {
		apiKey.MonthlyBudgetUSD = req.MonthlyBudgetUSD
	}

	if req.Enabled != nil {
		apiKey.Enabled = *req.Enabled
	}

	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" || *req.ExpiresAt == "null" {
			// Remove expiration
			apiKey.ExpiresAt = nil
		} else {
			parsedTime, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				utils.RespondWithError(w, http.StatusBadRequest, "Invalid expires_at format (use RFC3339)")
				return
			}
			apiKey.ExpiresAt = &parsedTime
		}
	}

	// Update in database
	if err := apiKeyRepo.Update(r.Context(), apiKey); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update API key")
		return
	}

	// Update tags if provided
	if req.Tags != nil {
		// Clear existing tags and set new ones
		for key, value := range req.Tags {
			if err := apiKeyRepo.SetTag(r.Context(), apiKey.ID, key, value); err != nil {
				// Log error but don't fail the request
				continue
			}
		}
		apiKey.Tags = req.Tags
	}

	response := h.toAPIKeyResponse(apiKey)
	utils.RespondWithJSON(w, http.StatusOK, response)
}

// Delete handles DELETE /admin/keys/:id - Revoke API key
func (h *AdminAPIKeysHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Extract key ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}
	keyIDStr := pathParts[2]

	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID format")
		return
	}

	apiKeyRepo := storage.NewAPIKeyRepository(h.db)
	apiKey, err := apiKeyRepo.GetByID(r.Context(), keyID)
	if err != nil {
		if err == storage.ErrAPIKeyNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "API key not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get API key")
		return
	}

	// Soft delete: set enabled to false
	apiKey.Enabled = false
	if err := apiKeyRepo.Update(r.Context(), apiKey); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to revoke API key")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "API key revoked successfully",
	})
}

// Regenerate handles POST /admin/keys/:id/regenerate - Generate new key, revoke old
func (h *AdminAPIKeysHandler) Regenerate(w http.ResponseWriter, r *http.Request) {
	// Extract key ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}
	keyIDStr := pathParts[2]

	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID format")
		return
	}

	apiKeyRepo := storage.NewAPIKeyRepository(h.db)
	oldKey, err := apiKeyRepo.GetByID(r.Context(), keyID)
	if err != nil {
		if err == storage.ErrAPIKeyNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "API key not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get API key")
		return
	}

	// Generate new API key
	plaintextKey, err := generateAPIKey()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	// Hash the new key
	keyHash := hashAPIKey(plaintextKey)

	// Update the key hash
	oldKey.KeyHash = keyHash

	if err := apiKeyRepo.Update(r.Context(), oldKey); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to regenerate API key")
		return
	}

	// Return response with new plaintext key (ONLY TIME IT'S VISIBLE)
	response := &APIKeyCreatedResponse{
		APIKeyResponse: h.toAPIKeyResponse(oldKey),
		Key:            plaintextKey,
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// toAPIKeyResponse converts a models.APIKey to APIKeyResponse
func (h *AdminAPIKeysHandler) toAPIKeyResponse(key *models.APIKey) APIKeyResponse {
	response := APIKeyResponse{
		ID:                 key.ID.String(),
		Name:               key.Name,
		AllowedModels:      []string(key.AllowedModels),
		RateLimitPerMinute: key.RateLimitPerMinute,
		MonthlyBudgetUSD:   key.MonthlyBudgetUSD,
		Enabled:            key.Enabled,
		CreatedAt:          key.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          key.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if key.ExpiresAt != nil {
		expiresAt := key.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
		response.ExpiresAt = &expiresAt
	}

	if key.Tags != nil && len(key.Tags) > 0 {
		response.Tags = key.Tags
	}

	return response
}

// getUsageStats retrieves usage statistics for an API key
func (h *AdminAPIKeysHandler) getUsageStats(ctx context.Context, keyID uuid.UUID) UsageStats {
	// Get current month's date range
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	usageRepo := storage.NewUsageRepository(h.db)

	// Get total cost for current month
	totalCost, err := usageRepo.GetTotalCostByAPIKey(ctx, keyID, startOfMonth, endOfMonth)
	if err != nil {
		totalCost = 0
	}

	// Get token usage for current month
	inputTokens, outputTokens, totalTokens, err := usageRepo.GetTotalTokensByAPIKey(ctx, keyID, startOfMonth, endOfMonth)
	if err != nil {
		inputTokens, outputTokens, totalTokens = 0, 0, 0
	}

	// Get usage records to calculate total requests and last used
	records, err := usageRepo.GetByAPIKey(ctx, keyID, startOfMonth, endOfMonth, 1, 0)
	var lastUsedAt *string
	totalRequests := 0

	if err == nil {
		totalRequests = len(records)
		if len(records) > 0 {
			lastUsed := records[0].CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			lastUsedAt = &lastUsed
		}
	}

	return UsageStats{
		TotalRequests:   totalRequests,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		TotalTokens:     totalTokens,
		CurrentMonthUSD: totalCost,
		LastUsedAt:      lastUsedAt,
	}
}
