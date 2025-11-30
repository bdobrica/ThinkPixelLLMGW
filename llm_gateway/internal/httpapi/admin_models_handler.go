package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

// AdminModelsHandler handles model management endpoints
type AdminModelsHandler struct {
	db       *storage.DB
	registry providers.Registry
}

// NewAdminModelsHandler creates a new admin models handler
func NewAdminModelsHandler(db *storage.DB, registry providers.Registry) *AdminModelsHandler {
	return &AdminModelsHandler{
		db:       db,
		registry: registry,
	}
}

// CreateModelRequest represents the request to create a new model
type CreateModelRequest struct {
	ModelName  string `json:"model_name"`
	ProviderID string `json:"provider_id"`
	Source     string `json:"source"`
	Version    string `json:"version,omitempty"`

	// Regions & resolutions
	SupportedRegions     []string `json:"supported_regions,omitempty"`
	SupportedResolutions []string `json:"supported_resolutions,omitempty"`

	// Feature support flags
	SupportsAssistantPrefill        bool `json:"supports_assistant_prefill,omitempty"`
	SupportsAudioInput              bool `json:"supports_audio_input,omitempty"`
	SupportsAudioOutput             bool `json:"supports_audio_output,omitempty"`
	SupportsComputerUse             bool `json:"supports_computer_use,omitempty"`
	SupportsEmbeddingImageInput     bool `json:"supports_embedding_image_input,omitempty"`
	SupportsFunctionCalling         bool `json:"supports_function_calling,omitempty"`
	SupportsImageInput              bool `json:"supports_image_input,omitempty"`
	SupportsNativeStreaming         bool `json:"supports_native_streaming,omitempty"`
	SupportsParallelFunctionCalling bool `json:"supports_parallel_function_calling,omitempty"`
	SupportsPDFInput                bool `json:"supports_pdf_input,omitempty"`
	SupportsPromptCaching           bool `json:"supports_prompt_caching,omitempty"`
	SupportsReasoning               bool `json:"supports_reasoning,omitempty"`
	SupportsResponseSchema          bool `json:"supports_response_schema,omitempty"`
	SupportsServiceTier             bool `json:"supports_service_tier,omitempty"`
	SupportsSystemMessages          bool `json:"supports_system_messages,omitempty"`
	SupportsToolChoice              bool `json:"supports_tool_choice,omitempty"`
	SupportsURLContext              bool `json:"supports_url_context,omitempty"`
	SupportsVideoInput              bool `json:"supports_video_input,omitempty"`
	SupportsVision                  bool `json:"supports_vision,omitempty"`
	SupportsWebSearch               bool `json:"supports_web_search,omitempty"`
	SupportsTextInput               bool `json:"supports_text_input,omitempty"`
	SupportsTextOutput              bool `json:"supports_text_output,omitempty"`
	SupportsImageOutput             bool `json:"supports_image_output,omitempty"`
	SupportsVideoOutput             bool `json:"supports_video_output,omitempty"`
	SupportsBatchRequests           bool `json:"supports_batch_requests,omitempty"`
	SupportsJSONOutput              bool `json:"supports_json_output,omitempty"`
	SupportsRerank                  bool `json:"supports_rerank,omitempty"`
	SupportsEmbeddingTextInput      bool `json:"supports_embedding_text_input,omitempty"`
	SupportsStreamingOutput         bool `json:"supports_streaming_output,omitempty"`

	// Limits & quotas
	TokensPerMinute   int `json:"tokens_per_minute,omitempty"`
	RequestsPerMinute int `json:"requests_per_minute,omitempty"`
	RequestsPerDay    int `json:"requests_per_day,omitempty"`

	// Token & context limits
	MaxTokens                 int `json:"max_tokens,omitempty"`
	MaxInputTokens            int `json:"max_input_tokens,omitempty"`
	MaxOutputTokens           int `json:"max_output_tokens,omitempty"`
	MaxQueryTokens            int `json:"max_query_tokens,omitempty"`
	MaxTokensPerDocumentChunk int `json:"max_tokens_per_document_chunk,omitempty"`
	MaxDocumentChunksPerQuery int `json:"max_document_chunks_per_query,omitempty"`
	ToolUseSystemPromptTokens int `json:"tool_use_system_prompt_tokens,omitempty"`
	OutputVectorSize          int `json:"output_vector_size,omitempty"`

	// Modality-specific limits
	MaxAudioLengthHours float64 `json:"max_audio_length_hours,omitempty"`
	MaxAudioPerPrompt   int     `json:"max_audio_per_prompt,omitempty"`
	MaxImagesPerPrompt  int     `json:"max_images_per_prompt,omitempty"`
	MaxPDFSizeMB        int     `json:"max_pdf_size_mb,omitempty"`
	MaxVideoLength      int     `json:"max_video_length,omitempty"`
	MaxVideosPerPrompt  int     `json:"max_videos_per_prompt,omitempty"`

	// Extended limits
	MaxRequestsPerSecond      int `json:"max_requests_per_second,omitempty"`
	MaxConcurrentRequests     int `json:"max_concurrent_requests,omitempty"`
	MaxBatchSize              int `json:"max_batch_size,omitempty"`
	MaxAudioLengthSeconds     int `json:"max_audio_length_seconds,omitempty"`
	MaxVideoLengthSeconds     int `json:"max_video_length_seconds,omitempty"`
	MaxContextWindowTokens    int `json:"max_context_window_tokens,omitempty"`
	MaxOutputTokensPerRequest int `json:"max_output_tokens_per_request,omitempty"`
	MaxInputTokensPerRequest  int `json:"max_input_tokens_per_request,omitempty"`

	// Pricing
	Currency                      string                   `json:"currency,omitempty"`
	PricingComponentSchemaVersion string                   `json:"pricing_component_schema_version,omitempty"`
	PricingComponents             []PricingComponentCreate `json:"pricing_components,omitempty"`

	// Operational metadata
	AverageLatencyMs float64 `json:"average_latency_ms,omitempty"`
	P95LatencyMs     float64 `json:"p95_latency_ms,omitempty"`
	AvailabilitySLO  float64 `json:"availability_slo,omitempty"`
	SLATier          string  `json:"sla_tier,omitempty"`
	SupportsSLA      bool    `json:"supports_sla,omitempty"`

	// Generic metadata
	MetadataSchemaVersion string                 `json:"metadata_schema_version,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
}

// PricingComponentCreate represents pricing component data for creation
type PricingComponentCreate struct {
	Code      string                 `json:"code"`
	Direction string                 `json:"direction"`
	Modality  string                 `json:"modality"`
	Unit      string                 `json:"unit"`
	Tier      string                 `json:"tier,omitempty"`
	Scope     string                 `json:"scope,omitempty"`
	Price     float64                `json:"price"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateModelRequest represents the request to update a model
type UpdateModelRequest struct {
	Version          *string  `json:"version,omitempty"`
	DeprecationDate  *string  `json:"deprecation_date,omitempty"` // ISO 8601 format
	IsDeprecated     *bool    `json:"is_deprecated,omitempty"`
	Currency         *string  `json:"currency,omitempty"`
	AverageLatencyMs *float64 `json:"average_latency_ms,omitempty"`
	P95LatencyMs     *float64 `json:"p95_latency_ms,omitempty"`
	AvailabilitySLO  *float64 `json:"availability_slo,omitempty"`
	SLATier          *string  `json:"sla_tier,omitempty"`
	SupportsSLA      *bool    `json:"supports_sla,omitempty"`

	// Allow updating pricing components
	PricingComponents *[]PricingComponentCreate `json:"pricing_components,omitempty"`

	// Allow updating metadata
	Metadata *map[string]interface{} `json:"metadata,omitempty"`
}

// ModelResponse represents a model response (summary view)
type ModelResponse struct {
	ID           string   `json:"id"`
	ModelName    string   `json:"model_name"`
	ProviderID   string   `json:"provider_id"`
	ProviderName string   `json:"provider_name"`
	Source       string   `json:"source"`
	Version      string   `json:"version,omitempty"`
	IsDeprecated bool     `json:"is_deprecated"`
	Currency     string   `json:"currency"`
	Features     []string `json:"features"` // Summary of enabled features
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// ModelDetailResponse represents a detailed model response
type ModelDetailResponse struct {
	ID         string `json:"id"`
	ModelName  string `json:"model_name"`
	ProviderID string `json:"provider_id"`
	Source     string `json:"source"`
	Version    string `json:"version,omitempty"`

	DeprecationDate *string `json:"deprecation_date,omitempty"`
	IsDeprecated    bool    `json:"is_deprecated"`

	// Regions & resolutions
	SupportedRegions     []string `json:"supported_regions,omitempty"`
	SupportedResolutions []string `json:"supported_resolutions,omitempty"`

	// Feature support flags
	SupportsAssistantPrefill        bool `json:"supports_assistant_prefill"`
	SupportsAudioInput              bool `json:"supports_audio_input"`
	SupportsAudioOutput             bool `json:"supports_audio_output"`
	SupportsComputerUse             bool `json:"supports_computer_use"`
	SupportsEmbeddingImageInput     bool `json:"supports_embedding_image_input"`
	SupportsFunctionCalling         bool `json:"supports_function_calling"`
	SupportsImageInput              bool `json:"supports_image_input"`
	SupportsNativeStreaming         bool `json:"supports_native_streaming"`
	SupportsParallelFunctionCalling bool `json:"supports_parallel_function_calling"`
	SupportsPDFInput                bool `json:"supports_pdf_input"`
	SupportsPromptCaching           bool `json:"supports_prompt_caching"`
	SupportsReasoning               bool `json:"supports_reasoning"`
	SupportsResponseSchema          bool `json:"supports_response_schema"`
	SupportsServiceTier             bool `json:"supports_service_tier"`
	SupportsSystemMessages          bool `json:"supports_system_messages"`
	SupportsToolChoice              bool `json:"supports_tool_choice"`
	SupportsURLContext              bool `json:"supports_url_context"`
	SupportsVideoInput              bool `json:"supports_video_input"`
	SupportsVision                  bool `json:"supports_vision"`
	SupportsWebSearch               bool `json:"supports_web_search"`
	SupportsTextInput               bool `json:"supports_text_input"`
	SupportsTextOutput              bool `json:"supports_text_output"`
	SupportsImageOutput             bool `json:"supports_image_output"`
	SupportsVideoOutput             bool `json:"supports_video_output"`
	SupportsBatchRequests           bool `json:"supports_batch_requests"`
	SupportsJSONOutput              bool `json:"supports_json_output"`
	SupportsRerank                  bool `json:"supports_rerank"`
	SupportsEmbeddingTextInput      bool `json:"supports_embedding_text_input"`
	SupportsStreamingOutput         bool `json:"supports_streaming_output"`

	// Limits & quotas
	TokensPerMinute   int `json:"tokens_per_minute"`
	RequestsPerMinute int `json:"requests_per_minute"`
	RequestsPerDay    int `json:"requests_per_day"`

	// Token & context limits
	MaxTokens                 int `json:"max_tokens"`
	MaxInputTokens            int `json:"max_input_tokens"`
	MaxOutputTokens           int `json:"max_output_tokens"`
	MaxQueryTokens            int `json:"max_query_tokens"`
	MaxTokensPerDocumentChunk int `json:"max_tokens_per_document_chunk"`
	MaxDocumentChunksPerQuery int `json:"max_document_chunks_per_query"`
	ToolUseSystemPromptTokens int `json:"tool_use_system_prompt_tokens"`
	OutputVectorSize          int `json:"output_vector_size"`

	// Modality-specific limits
	MaxAudioLengthHours float64 `json:"max_audio_length_hours"`
	MaxAudioPerPrompt   int     `json:"max_audio_per_prompt"`
	MaxImagesPerPrompt  int     `json:"max_images_per_prompt"`
	MaxPDFSizeMB        int     `json:"max_pdf_size_mb"`
	MaxVideoLength      int     `json:"max_video_length"`
	MaxVideosPerPrompt  int     `json:"max_videos_per_prompt"`

	// Extended limits
	MaxRequestsPerSecond      int `json:"max_requests_per_second"`
	MaxConcurrentRequests     int `json:"max_concurrent_requests"`
	MaxBatchSize              int `json:"max_batch_size"`
	MaxAudioLengthSeconds     int `json:"max_audio_length_seconds"`
	MaxVideoLengthSeconds     int `json:"max_video_length_seconds"`
	MaxContextWindowTokens    int `json:"max_context_window_tokens"`
	MaxOutputTokensPerRequest int `json:"max_output_tokens_per_request"`
	MaxInputTokensPerRequest  int `json:"max_input_tokens_per_request"`

	// Pricing
	Currency                      string                     `json:"currency"`
	PricingComponentSchemaVersion string                     `json:"pricing_component_schema_version,omitempty"`
	PricingComponents             []PricingComponentResponse `json:"pricing_components"`

	// Operational metadata
	AverageLatencyMs float64 `json:"average_latency_ms"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	AvailabilitySLO  float64 `json:"availability_slo"`
	SLATier          string  `json:"sla_tier,omitempty"`
	SupportsSLA      bool    `json:"supports_sla"`

	// Generic metadata
	MetadataSchemaVersion string                 `json:"metadata_schema_version,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`

	// Timestamps
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// Related data
	AliasCount int `json:"alias_count"`
}

// PricingComponentResponse represents a pricing component in responses
type PricingComponentResponse struct {
	ID        string                 `json:"id"`
	Code      string                 `json:"code"`
	Direction string                 `json:"direction"`
	Modality  string                 `json:"modality"`
	Unit      string                 `json:"unit"`
	Tier      string                 `json:"tier,omitempty"`
	Scope     string                 `json:"scope,omitempty"`
	Price     float64                `json:"price"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Create handles POST /admin/models - Create new model
func (h *AdminModelsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if req.ModelName == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Model name is required")
		return
	}
	if req.ProviderID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Provider ID is required")
		return
	}
	if req.Source == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Source is required")
		return
	}

	// Validate provider exists and is enabled
	providerRepo := storage.NewProviderRepository(h.db)
	providerUUID, err := uuid.Parse(req.ProviderID)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid provider ID format")
		return
	}

	provider, err := providerRepo.GetByID(r.Context(), providerUUID)
	if err != nil {
		if err == storage.ErrProviderNotFound {
			utils.RespondWithError(w, http.StatusNotFound, "Provider not found")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to validate provider")
		return
	}

	if !provider.Enabled {
		utils.RespondWithError(w, http.StatusBadRequest, "Provider is not enabled")
		return
	}

	// Create model
	model := &models.Model{
		ID:         uuid.New(),
		ModelName:  req.ModelName,
		ProviderID: req.ProviderID,
		Source:     req.Source,
		Version:    req.Version,

		SupportedRegions:     pq.StringArray(req.SupportedRegions),
		SupportedResolutions: pq.StringArray(req.SupportedResolutions),

		SupportsAssistantPrefill:        req.SupportsAssistantPrefill,
		SupportsAudioInput:              req.SupportsAudioInput,
		SupportsAudioOutput:             req.SupportsAudioOutput,
		SupportsComputerUse:             req.SupportsComputerUse,
		SupportsEmbeddingImageInput:     req.SupportsEmbeddingImageInput,
		SupportsFunctionCalling:         req.SupportsFunctionCalling,
		SupportsImageInput:              req.SupportsImageInput,
		SupportsNativeStreaming:         req.SupportsNativeStreaming,
		SupportsParallelFunctionCalling: req.SupportsParallelFunctionCalling,
		SupportsPDFInput:                req.SupportsPDFInput,
		SupportsPromptCaching:           req.SupportsPromptCaching,
		SupportsReasoning:               req.SupportsReasoning,
		SupportsResponseSchema:          req.SupportsResponseSchema,
		SupportsServiceTier:             req.SupportsServiceTier,
		SupportsSystemMessages:          req.SupportsSystemMessages,
		SupportsToolChoice:              req.SupportsToolChoice,
		SupportsURLContext:              req.SupportsURLContext,
		SupportsVideoInput:              req.SupportsVideoInput,
		SupportsVision:                  req.SupportsVision,
		SupportsWebSearch:               req.SupportsWebSearch,
		SupportsTextInput:               req.SupportsTextInput,
		SupportsTextOutput:              req.SupportsTextOutput,
		SupportsImageOutput:             req.SupportsImageOutput,
		SupportsVideoOutput:             req.SupportsVideoOutput,
		SupportsBatchRequests:           req.SupportsBatchRequests,
		SupportsJSONOutput:              req.SupportsJSONOutput,
		SupportsRerank:                  req.SupportsRerank,
		SupportsEmbeddingTextInput:      req.SupportsEmbeddingTextInput,
		SupportsStreamingOutput:         req.SupportsStreamingOutput,

		TokensPerMinute:   req.TokensPerMinute,
		RequestsPerMinute: req.RequestsPerMinute,
		RequestsPerDay:    req.RequestsPerDay,

		MaxTokens:                 req.MaxTokens,
		MaxInputTokens:            req.MaxInputTokens,
		MaxOutputTokens:           req.MaxOutputTokens,
		MaxQueryTokens:            req.MaxQueryTokens,
		MaxTokensPerDocumentChunk: req.MaxTokensPerDocumentChunk,
		MaxDocumentChunksPerQuery: req.MaxDocumentChunksPerQuery,
		ToolUseSystemPromptTokens: req.ToolUseSystemPromptTokens,
		OutputVectorSize:          req.OutputVectorSize,

		MaxAudioLengthHours: req.MaxAudioLengthHours,
		MaxAudioPerPrompt:   req.MaxAudioPerPrompt,
		MaxImagesPerPrompt:  req.MaxImagesPerPrompt,
		MaxPDFSizeMB:        req.MaxPDFSizeMB,
		MaxVideoLength:      req.MaxVideoLength,
		MaxVideosPerPrompt:  req.MaxVideosPerPrompt,

		MaxRequestsPerSecond:      req.MaxRequestsPerSecond,
		MaxConcurrentRequests:     req.MaxConcurrentRequests,
		MaxBatchSize:              req.MaxBatchSize,
		MaxAudioLengthSeconds:     req.MaxAudioLengthSeconds,
		MaxVideoLengthSeconds:     req.MaxVideoLengthSeconds,
		MaxContextWindowTokens:    req.MaxContextWindowTokens,
		MaxOutputTokensPerRequest: req.MaxOutputTokensPerRequest,
		MaxInputTokensPerRequest:  req.MaxInputTokensPerRequest,

		Currency:         req.Currency,
		AverageLatencyMs: req.AverageLatencyMs,
		P95LatencyMs:     req.P95LatencyMs,
		AvailabilitySLO:  req.AvailabilitySLO,
		SupportsSLA:      req.SupportsSLA,
	}

	if req.PricingComponentSchemaVersion != "" {
		model.PricingComponentSchemaVersion = &req.PricingComponentSchemaVersion
	}
	if req.SLATier != "" {
		model.SLATier = &req.SLATier
	}
	if req.MetadataSchemaVersion != "" {
		model.MetadataSchemaVersion = &req.MetadataSchemaVersion
	}
	if req.Metadata != nil {
		model.Metadata = models.JSONB(req.Metadata)
	}

	// Create model in database
	if err := h.createModelWithPricing(r.Context(), model, req.PricingComponents); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			utils.RespondWithError(w, http.StatusConflict, "Model with this name already exists")
			return
		}
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create model")
		return
	}

	// Trigger registry reload
	if err := h.registry.Reload(r.Context()); err != nil {
		// Log error but don't fail the request
	}

	response := &ModelResponse{
		ID:           model.ID.String(),
		ModelName:    model.ModelName,
		ProviderID:   model.ProviderID,
		ProviderName: provider.Name,
		Source:       model.Source,
		Version:      model.Version,
		IsDeprecated: model.IsDeprecated,
		Currency:     model.Currency,
		Features:     extractFeatures(model),
		CreatedAt:    model.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    model.UpdatedAt.Format(time.RFC3339),
	}

	utils.RespondWithJSON(w, http.StatusCreated, response)
}

// createModelWithPricing creates a model and its pricing components in a transaction
func (h *AdminModelsHandler) createModelWithPricing(ctx context.Context, model *models.Model, pricingComponents []PricingComponentCreate) error {
	// Start transaction
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert model
	query := `
		INSERT INTO models (
			id, model_name, provider_id, source, version, is_deprecated,
			supported_regions, supported_resolutions,
			supports_assistant_prefill, supports_audio_input, supports_audio_output,
			supports_computer_use, supports_embedding_image_input, supports_function_calling,
			supports_image_input, supports_native_streaming, supports_parallel_function_calling,
			supports_pdf_input, supports_prompt_caching, supports_reasoning,
			supports_response_schema, supports_service_tier, supports_system_messages,
			supports_tool_choice, supports_url_context, supports_video_input,
			supports_vision, supports_web_search,
			supports_text_input, supports_text_output, supports_image_output,
			supports_video_output, supports_batch_requests, supports_json_output,
			supports_rerank, supports_embedding_text_input, supports_streaming_output,
			tokens_per_minute, requests_per_minute, requests_per_day,
			max_tokens, max_input_tokens, max_output_tokens, max_query_tokens,
			max_tokens_per_document_chunk, max_document_chunks_per_query,
			tool_use_system_prompt_tokens, output_vector_size,
			max_audio_length_hours, max_audio_per_prompt, max_images_per_prompt,
			max_pdf_size_mb, max_video_length, max_videos_per_prompt,
			max_requests_per_second, max_concurrent_requests, max_batch_size,
			max_audio_length_seconds, max_video_length_seconds,
			max_context_window_tokens, max_output_tokens_per_request,
			max_input_tokens_per_request,
			currency, pricing_component_schema_version,
			average_latency_ms, p95_latency_ms, availability_slo, sla_tier, supports_sla,
			metadata_schema_version, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38,
			$39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50, $51, $52, $53, $54, $55, $56,
			$57, $58, $59, $60, $61, $62, $63, $64, $65
		)
	`

	_, err = tx.ExecContext(ctx, query,
		model.ID, model.ModelName, model.ProviderID, model.Source, model.Version, model.IsDeprecated,
		model.SupportedRegions, model.SupportedResolutions,
		model.SupportsAssistantPrefill, model.SupportsAudioInput, model.SupportsAudioOutput,
		model.SupportsComputerUse, model.SupportsEmbeddingImageInput, model.SupportsFunctionCalling,
		model.SupportsImageInput, model.SupportsNativeStreaming, model.SupportsParallelFunctionCalling,
		model.SupportsPDFInput, model.SupportsPromptCaching, model.SupportsReasoning,
		model.SupportsResponseSchema, model.SupportsServiceTier, model.SupportsSystemMessages,
		model.SupportsToolChoice, model.SupportsURLContext, model.SupportsVideoInput,
		model.SupportsVision, model.SupportsWebSearch,
		model.SupportsTextInput, model.SupportsTextOutput, model.SupportsImageOutput,
		model.SupportsVideoOutput, model.SupportsBatchRequests, model.SupportsJSONOutput,
		model.SupportsRerank, model.SupportsEmbeddingTextInput, model.SupportsStreamingOutput,
		model.TokensPerMinute, model.RequestsPerMinute, model.RequestsPerDay,
		model.MaxTokens, model.MaxInputTokens, model.MaxOutputTokens, model.MaxQueryTokens,
		model.MaxTokensPerDocumentChunk, model.MaxDocumentChunksPerQuery,
		model.ToolUseSystemPromptTokens, model.OutputVectorSize,
		model.MaxAudioLengthHours, model.MaxAudioPerPrompt, model.MaxImagesPerPrompt,
		model.MaxPDFSizeMB, model.MaxVideoLength, model.MaxVideosPerPrompt,
		model.MaxRequestsPerSecond, model.MaxConcurrentRequests, model.MaxBatchSize,
		model.MaxAudioLengthSeconds, model.MaxVideoLengthSeconds,
		model.MaxContextWindowTokens, model.MaxOutputTokensPerRequest,
		model.MaxInputTokensPerRequest,
		model.Currency, model.PricingComponentSchemaVersion,
		model.AverageLatencyMs, model.P95LatencyMs, model.AvailabilitySLO, model.SLATier, model.SupportsSLA,
		model.MetadataSchemaVersion, model.Metadata,
	)
	if err != nil {
		return err
	}

	// Insert pricing components
	if len(pricingComponents) > 0 {
		pricingQuery := `
			INSERT INTO pricing_components (
				id, model_id, code, direction, modality, unit, tier, scope, price,
				metadata_schema_version, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`

		for _, pc := range pricingComponents {
			pcID := uuid.New()
			var metadata models.JSONB
			if pc.Metadata != nil {
				metadata = models.JSONB(pc.Metadata)
			}

			_, err = tx.ExecContext(ctx, pricingQuery,
				pcID, model.ID, pc.Code, pc.Direction, pc.Modality, pc.Unit,
				pc.Tier, pc.Scope, pc.Price, nil, metadata,
			)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// List handles GET /admin/models - List all models
func (h *AdminModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()

	providerID := query.Get("provider_id")
	search := query.Get("search")

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

	// Create filters
	filters := storage.ModelListFilters{
		ProviderID: providerID,
		Search:     search,
		Page:       page,
		PageSize:   pageSize,
	}

	modelRepo := storage.NewModelRepository(h.db)
	providerRepo := storage.NewProviderRepository(h.db)

	result, err := modelRepo.ListWithFilters(r.Context(), filters)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list models")
		return
	}

	// Build responses
	responses := make([]ModelResponse, 0, len(result.Models))
	for _, m := range result.Models {
		// Get provider name
		providerName := m.ProviderID
		providerUUID, err := uuid.Parse(m.ProviderID)
		if err == nil {
			if provider, err := providerRepo.GetByID(r.Context(), providerUUID); err == nil {
				providerName = provider.Name
			}
		}

		responses = append(responses, ModelResponse{
			ID:           m.ID.String(),
			ModelName:    m.ModelName,
			ProviderID:   m.ProviderID,
			ProviderName: providerName,
			Source:       m.Source,
			Version:      m.Version,
			IsDeprecated: m.IsDeprecated,
			Currency:     m.Currency,
			Features:     extractFeatures(m),
			CreatedAt:    m.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    m.UpdatedAt.Format(time.RFC3339),
		})
	}

	// Return paginated response
	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"items":       responses,
		"total_count": result.TotalCount,
		"page":        result.Page,
		"page_size":   result.PageSize,
	})
}

// GetByID handles GET /admin/models/:id - Get model details
func (h *AdminModelsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	// Extract model ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid model ID")
		return
	}
	modelIDStr := pathParts[2]

	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid model ID format")
		return
	}

	modelRepo := storage.NewModelRepository(h.db)
	model, err := modelRepo.GetByID(r.Context(), modelID)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Model not found")
		return
	}

	// Get alias count
	aliasRepo := storage.NewModelAliasRepository(h.db)
	aliases, _ := aliasRepo.ListByModel(r.Context(), modelID)

	// Build pricing components response
	pricingComponents := make([]PricingComponentResponse, 0, len(model.PricingComponents))
	for _, pc := range model.PricingComponents {
		var metadata map[string]interface{}
		if pc.Metadata != nil {
			metadata = pc.Metadata
		}

		pricingComponents = append(pricingComponents, PricingComponentResponse{
			ID:        pc.ID,
			Code:      pc.Code,
			Direction: string(pc.Direction),
			Modality:  string(pc.Modality),
			Unit:      string(pc.Unit),
			Tier:      utils.StringPtrValue(pc.Tier),
			Scope:     utils.StringPtrValue(pc.Scope),
			Price:     pc.Price,
			Metadata:  metadata,
		})
	}

	var metadata map[string]interface{}
	if model.Metadata != nil {
		metadata = model.Metadata
	}

	var deprecationDate *string
	if model.DeprecationDate != nil {
		formatted := model.DeprecationDate.Format(time.RFC3339)
		deprecationDate = &formatted
	}

	response := &ModelDetailResponse{
		ID:         model.ID.String(),
		ModelName:  model.ModelName,
		ProviderID: model.ProviderID,
		Source:     model.Source,
		Version:    model.Version,

		DeprecationDate: deprecationDate,
		IsDeprecated:    model.IsDeprecated,

		SupportedRegions:     model.SupportedRegions,
		SupportedResolutions: model.SupportedResolutions,

		SupportsAssistantPrefill:        model.SupportsAssistantPrefill,
		SupportsAudioInput:              model.SupportsAudioInput,
		SupportsAudioOutput:             model.SupportsAudioOutput,
		SupportsComputerUse:             model.SupportsComputerUse,
		SupportsEmbeddingImageInput:     model.SupportsEmbeddingImageInput,
		SupportsFunctionCalling:         model.SupportsFunctionCalling,
		SupportsImageInput:              model.SupportsImageInput,
		SupportsNativeStreaming:         model.SupportsNativeStreaming,
		SupportsParallelFunctionCalling: model.SupportsParallelFunctionCalling,
		SupportsPDFInput:                model.SupportsPDFInput,
		SupportsPromptCaching:           model.SupportsPromptCaching,
		SupportsReasoning:               model.SupportsReasoning,
		SupportsResponseSchema:          model.SupportsResponseSchema,
		SupportsServiceTier:             model.SupportsServiceTier,
		SupportsSystemMessages:          model.SupportsSystemMessages,
		SupportsToolChoice:              model.SupportsToolChoice,
		SupportsURLContext:              model.SupportsURLContext,
		SupportsVideoInput:              model.SupportsVideoInput,
		SupportsVision:                  model.SupportsVision,
		SupportsWebSearch:               model.SupportsWebSearch,
		SupportsTextInput:               model.SupportsTextInput,
		SupportsTextOutput:              model.SupportsTextOutput,
		SupportsImageOutput:             model.SupportsImageOutput,
		SupportsVideoOutput:             model.SupportsVideoOutput,
		SupportsBatchRequests:           model.SupportsBatchRequests,
		SupportsJSONOutput:              model.SupportsJSONOutput,
		SupportsRerank:                  model.SupportsRerank,
		SupportsEmbeddingTextInput:      model.SupportsEmbeddingTextInput,
		SupportsStreamingOutput:         model.SupportsStreamingOutput,

		TokensPerMinute:   model.TokensPerMinute,
		RequestsPerMinute: model.RequestsPerMinute,
		RequestsPerDay:    model.RequestsPerDay,

		MaxTokens:                 model.MaxTokens,
		MaxInputTokens:            model.MaxInputTokens,
		MaxOutputTokens:           model.MaxOutputTokens,
		MaxQueryTokens:            model.MaxQueryTokens,
		MaxTokensPerDocumentChunk: model.MaxTokensPerDocumentChunk,
		MaxDocumentChunksPerQuery: model.MaxDocumentChunksPerQuery,
		ToolUseSystemPromptTokens: model.ToolUseSystemPromptTokens,
		OutputVectorSize:          model.OutputVectorSize,

		MaxAudioLengthHours: model.MaxAudioLengthHours,
		MaxAudioPerPrompt:   model.MaxAudioPerPrompt,
		MaxImagesPerPrompt:  model.MaxImagesPerPrompt,
		MaxPDFSizeMB:        model.MaxPDFSizeMB,
		MaxVideoLength:      model.MaxVideoLength,
		MaxVideosPerPrompt:  model.MaxVideosPerPrompt,

		MaxRequestsPerSecond:      model.MaxRequestsPerSecond,
		MaxConcurrentRequests:     model.MaxConcurrentRequests,
		MaxBatchSize:              model.MaxBatchSize,
		MaxAudioLengthSeconds:     model.MaxAudioLengthSeconds,
		MaxVideoLengthSeconds:     model.MaxVideoLengthSeconds,
		MaxContextWindowTokens:    model.MaxContextWindowTokens,
		MaxOutputTokensPerRequest: model.MaxOutputTokensPerRequest,
		MaxInputTokensPerRequest:  model.MaxInputTokensPerRequest,

		Currency:                      model.Currency,
		PricingComponentSchemaVersion: utils.StringPtrValue(model.PricingComponentSchemaVersion),
		PricingComponents:             pricingComponents,

		AverageLatencyMs: model.AverageLatencyMs,
		P95LatencyMs:     model.P95LatencyMs,
		AvailabilitySLO:  model.AvailabilitySLO,
		SLATier:          utils.StringPtrValue(model.SLATier),
		SupportsSLA:      model.SupportsSLA,

		MetadataSchemaVersion: utils.StringPtrValue(model.MetadataSchemaVersion),
		Metadata:              metadata,

		CreatedAt: model.CreatedAt.Format(time.RFC3339),
		UpdatedAt: model.UpdatedAt.Format(time.RFC3339),

		AliasCount: len(aliases),
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// Update handles PUT /admin/models/:id - Update model
func (h *AdminModelsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Extract model ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid model ID")
		return
	}
	modelIDStr := pathParts[2]

	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid model ID format")
		return
	}

	var req UpdateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	modelRepo := storage.NewModelRepository(h.db)
	model, err := modelRepo.GetByID(r.Context(), modelID)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Model not found")
		return
	}

	// Update fields if provided
	if req.Version != nil {
		model.Version = *req.Version
	}

	if req.DeprecationDate != nil {
		if *req.DeprecationDate == "" {
			model.DeprecationDate = nil
		} else {
			parsed, err := time.Parse(time.RFC3339, *req.DeprecationDate)
			if err != nil {
				utils.RespondWithError(w, http.StatusBadRequest, "Invalid deprecation_date format (use RFC3339)")
				return
			}
			model.DeprecationDate = &parsed
		}
	}

	if req.IsDeprecated != nil {
		model.IsDeprecated = *req.IsDeprecated
	}

	if req.Currency != nil {
		model.Currency = *req.Currency
	}

	if req.AverageLatencyMs != nil {
		model.AverageLatencyMs = *req.AverageLatencyMs
	}

	if req.P95LatencyMs != nil {
		model.P95LatencyMs = *req.P95LatencyMs
	}

	if req.AvailabilitySLO != nil {
		model.AvailabilitySLO = *req.AvailabilitySLO
	}

	if req.SLATier != nil {
		model.SLATier = req.SLATier
	}

	if req.SupportsSLA != nil {
		model.SupportsSLA = *req.SupportsSLA
	}

	if req.Metadata != nil {
		model.Metadata = models.JSONB(*req.Metadata)
	}

	// Update model and pricing components if needed
	if req.PricingComponents != nil {
		if err := h.updateModelWithPricing(r.Context(), model, *req.PricingComponents); err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update model")
			return
		}
	} else {
		// Just update the model
		if err := h.updateModelOnly(r.Context(), model); err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update model")
			return
		}
	}

	// Invalidate model cache
	modelRepo.InvalidateCache(model.ModelName)

	// Trigger registry reload
	if err := h.registry.Reload(r.Context()); err != nil {
		// Log error but don't fail the request
	}

	response := &ModelResponse{
		ID:           model.ID.String(),
		ModelName:    model.ModelName,
		ProviderID:   model.ProviderID,
		Source:       model.Source,
		Version:      model.Version,
		IsDeprecated: model.IsDeprecated,
		Currency:     model.Currency,
		Features:     extractFeatures(model),
		CreatedAt:    model.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    model.UpdatedAt.Format(time.RFC3339),
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// updateModelOnly updates just the model record
func (h *AdminModelsHandler) updateModelOnly(ctx context.Context, model *models.Model) error {
	query := `
		UPDATE models SET
			version = $2,
			deprecation_date = $3,
			is_deprecated = $4,
			currency = $5,
			average_latency_ms = $6,
			p95_latency_ms = $7,
			availability_slo = $8,
			sla_tier = $9,
			supports_sla = $10,
			metadata = $11,
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := h.db.Conn().ExecContext(ctx, query,
		model.ID, model.Version, model.DeprecationDate, model.IsDeprecated,
		model.Currency, model.AverageLatencyMs, model.P95LatencyMs, model.AvailabilitySLO,
		model.SLATier, model.SupportsSLA, model.Metadata,
	)

	return err
}

// updateModelWithPricing updates model and replaces pricing components in a transaction
func (h *AdminModelsHandler) updateModelWithPricing(ctx context.Context, model *models.Model, pricingComponents []PricingComponentCreate) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update model
	query := `
		UPDATE models SET
			version = $2,
			deprecation_date = $3,
			is_deprecated = $4,
			currency = $5,
			average_latency_ms = $6,
			p95_latency_ms = $7,
			availability_slo = $8,
			sla_tier = $9,
			supports_sla = $10,
			metadata = $11,
			updated_at = NOW()
		WHERE id = $1
	`

	_, err = tx.ExecContext(ctx, query,
		model.ID, model.Version, model.DeprecationDate, model.IsDeprecated,
		model.Currency, model.AverageLatencyMs, model.P95LatencyMs, model.AvailabilitySLO,
		model.SLATier, model.SupportsSLA, model.Metadata,
	)
	if err != nil {
		return err
	}

	// Delete existing pricing components
	_, err = tx.ExecContext(ctx, "DELETE FROM pricing_components WHERE model_id = $1", model.ID)
	if err != nil {
		return err
	}

	// Insert new pricing components
	if len(pricingComponents) > 0 {
		pricingQuery := `
			INSERT INTO pricing_components (
				id, model_id, code, direction, modality, unit, tier, scope, price,
				metadata_schema_version, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`

		for _, pc := range pricingComponents {
			pcID := uuid.New()
			var metadata models.JSONB
			if pc.Metadata != nil {
				metadata = models.JSONB(pc.Metadata)
			}

			_, err = tx.ExecContext(ctx, pricingQuery,
				pcID, model.ID, pc.Code, pc.Direction, pc.Modality, pc.Unit,
				pc.Tier, pc.Scope, pc.Price, nil, metadata,
			)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// Delete handles DELETE /admin/models/:id - Delete model
func (h *AdminModelsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Extract model ID from URL path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid model ID")
		return
	}
	modelIDStr := pathParts[2]

	modelID, err := uuid.Parse(modelIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid model ID format")
		return
	}

	modelRepo := storage.NewModelRepository(h.db)
	model, err := modelRepo.GetByID(r.Context(), modelID)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Model not found")
		return
	}

	// Check for dependent aliases
	aliasRepo := storage.NewModelAliasRepository(h.db)
	aliases, _ := aliasRepo.ListByModel(r.Context(), modelID)

	if len(aliases) > 0 {
		utils.RespondWithError(w, http.StatusConflict, "Cannot delete model with active aliases")
		return
	}

	// Delete model (cascade will delete pricing components)
	if err := modelRepo.Delete(r.Context(), modelID); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete model")
		return
	}

	// Invalidate model cache
	modelRepo.InvalidateCache(model.ModelName)

	// Trigger registry reload
	if err := h.registry.Reload(r.Context()); err != nil {
		// Log error but don't fail the request
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Model deleted successfully",
	})
}

// Helper functions

func extractFeatures(m *models.Model) []string {
	features := []string{}

	if m.SupportsFunctionCalling {
		features = append(features, "function_calling")
	}
	if m.SupportsVision {
		features = append(features, "vision")
	}
	if m.SupportsNativeStreaming {
		features = append(features, "streaming")
	}
	if m.SupportsPromptCaching {
		features = append(features, "prompt_caching")
	}
	if m.SupportsReasoning {
		features = append(features, "reasoning")
	}
	if m.SupportsAudioInput || m.SupportsAudioOutput {
		features = append(features, "audio")
	}
	if m.SupportsVideoInput || m.SupportsVideoOutput {
		features = append(features, "video")
	}

	return features
}
