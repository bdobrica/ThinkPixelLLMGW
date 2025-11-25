package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

//
// Model (models table)
//

type Model struct {
	ID uuid.UUID `db:"id" json:"id"` // uuid

	// 1. Identity & lifecycle
	ModelName  string `db:"model_name" json:"model_name"`
	ProviderID string `db:"provider_id" json:"provider_id"`
	Source     string `db:"source" json:"source"`
	Version    string `db:"version" json:"version,omitempty"`

	DeprecationDate *time.Time `db:"deprecation_date" json:"deprecation_date,omitempty"`
	IsDeprecated    bool       `db:"is_deprecated" json:"is_deprecated"`

	// 2. Regions & resolutions
	SupportedRegions     pq.StringArray `db:"supported_regions" json:"supported_regions,omitempty"`
	SupportedResolutions pq.StringArray `db:"supported_resolutions" json:"supported_resolutions,omitempty"`

	// 3. Feature support flags (original set)
	SupportsAssistantPrefill        bool `db:"supports_assistant_prefill" json:"supports_assistant_prefill"`
	SupportsAudioInput              bool `db:"supports_audio_input" json:"supports_audio_input"`
	SupportsAudioOutput             bool `db:"supports_audio_output" json:"supports_audio_output"`
	SupportsComputerUse             bool `db:"supports_computer_use" json:"supports_computer_use"`
	SupportsEmbeddingImageInput     bool `db:"supports_embedding_image_input" json:"supports_embedding_image_input"`
	SupportsFunctionCalling         bool `db:"supports_function_calling" json:"supports_function_calling"`
	SupportsImageInput              bool `db:"supports_image_input" json:"supports_image_input"`
	SupportsNativeStreaming         bool `db:"supports_native_streaming" json:"supports_native_streaming"`
	SupportsParallelFunctionCalling bool `db:"supports_parallel_function_calling" json:"supports_parallel_function_calling"`
	SupportsPDFInput                bool `db:"supports_pdf_input" json:"supports_pdf_input"`
	SupportsPromptCaching           bool `db:"supports_prompt_caching" json:"supports_prompt_caching"`
	SupportsReasoning               bool `db:"supports_reasoning" json:"supports_reasoning"`
	SupportsResponseSchema          bool `db:"supports_response_schema" json:"supports_response_schema"`
	SupportsServiceTier             bool `db:"supports_service_tier" json:"supports_service_tier"`
	SupportsSystemMessages          bool `db:"supports_system_messages" json:"supports_system_messages"`
	SupportsToolChoice              bool `db:"supports_tool_choice" json:"supports_tool_choice"`
	SupportsURLContext              bool `db:"supports_url_context" json:"supports_url_context"`
	SupportsVideoInput              bool `db:"supports_video_input" json:"supports_video_input"`
	SupportsVision                  bool `db:"supports_vision" json:"supports_vision"`
	SupportsWebSearch               bool `db:"supports_web_search" json:"supports_web_search"`

	// 3b. Symmetric / extended flags
	SupportsTextInput          bool `db:"supports_text_input" json:"supports_text_input"`
	SupportsTextOutput         bool `db:"supports_text_output" json:"supports_text_output"`
	SupportsImageOutput        bool `db:"supports_image_output" json:"supports_image_output"`
	SupportsVideoOutput        bool `db:"supports_video_output" json:"supports_video_output"`
	SupportsBatchRequests      bool `db:"supports_batch_requests" json:"supports_batch_requests"`
	SupportsJSONOutput         bool `db:"supports_json_output" json:"supports_json_output"`
	SupportsRerank             bool `db:"supports_rerank" json:"supports_rerank"`
	SupportsEmbeddingTextInput bool `db:"supports_embedding_text_input" json:"supports_embedding_text_input"`
	SupportsStreamingOutput    bool `db:"supports_streaming_output" json:"supports_streaming_output"`

	// 4. Limits & quotas – throughput
	TokensPerMinute   int `db:"tokens_per_minute" json:"tokens_per_minute"`
	RequestsPerMinute int `db:"requests_per_minute" json:"requests_per_minute"`
	RequestsPerDay    int `db:"requests_per_day" json:"requests_per_day"`

	// 4b. Token & context limits
	MaxTokens                 int `db:"max_tokens" json:"max_tokens"`
	MaxInputTokens            int `db:"max_input_tokens" json:"max_input_tokens"`
	MaxOutputTokens           int `db:"max_output_tokens" json:"max_output_tokens"`
	MaxQueryTokens            int `db:"max_query_tokens" json:"max_query_tokens"`
	MaxTokensPerDocumentChunk int `db:"max_tokens_per_document_chunk" json:"max_tokens_per_document_chunk"`
	MaxDocumentChunksPerQuery int `db:"max_document_chunks_per_query" json:"max_document_chunks_per_query"`
	ToolUseSystemPromptTokens int `db:"tool_use_system_prompt_tokens" json:"tool_use_system_prompt_tokens"`
	OutputVectorSize          int `db:"output_vector_size" json:"output_vector_size"`

	// 4c. Modality-specific limits
	MaxAudioLengthHours float64 `db:"max_audio_length_hours" json:"max_audio_length_hours"`
	MaxAudioPerPrompt   int     `db:"max_audio_per_prompt" json:"max_audio_per_prompt"`
	MaxImagesPerPrompt  int     `db:"max_images_per_prompt" json:"max_images_per_prompt"`
	MaxPDFSizeMB        int     `db:"max_pdf_size_mb" json:"max_pdf_size_mb"`
	MaxVideoLength      int     `db:"max_video_length" json:"max_video_length"`
	MaxVideosPerPrompt  int     `db:"max_videos_per_prompt" json:"max_videos_per_prompt"`

	// 4d. Extended limits
	MaxRequestsPerSecond      int `db:"max_requests_per_second" json:"max_requests_per_second"`
	MaxConcurrentRequests     int `db:"max_concurrent_requests" json:"max_concurrent_requests"`
	MaxBatchSize              int `db:"max_batch_size" json:"max_batch_size"`
	MaxAudioLengthSeconds     int `db:"max_audio_length_seconds" json:"max_audio_length_seconds"`
	MaxVideoLengthSeconds     int `db:"max_video_length_seconds" json:"max_video_length_seconds"`
	MaxContextWindowTokens    int `db:"max_context_window_tokens" json:"max_context_window_tokens"`
	MaxOutputTokensPerRequest int `db:"max_output_tokens_per_request" json:"max_output_tokens_per_request"`
	MaxInputTokensPerRequest  int `db:"max_input_tokens_per_request" json:"max_input_tokens_per_request"`

	// 5. Pricing (normalized)
	Currency                      string  `db:"currency" json:"currency"`
	PricingComponentSchemaVersion *string `db:"pricing_component_schema_version" json:"pricing_component_schema_version,omitempty"`

	// 6. Operational metadata
	AverageLatencyMs float64 `db:"average_latency_ms" json:"average_latency_ms"`
	P95LatencyMs     float64 `db:"p95_latency_ms" json:"p95_latency_ms"`
	AvailabilitySLO  float64 `db:"availability_slo" json:"availability_slo"`
	SLATier          *string `db:"sla_tier" json:"sla_tier,omitempty"`
	SupportsSLA      bool    `db:"supports_sla" json:"supports_sla"`

	// 7. Generic metadata
	MetadataSchemaVersion *string `db:"metadata_schema_version" json:"metadata_schema_version,omitempty"`
	Metadata              JSONB   `db:"metadata" json:"metadata,omitempty"`

	// 8. Timestamps
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// Joined in code, not a DB column:
	PricingComponents []PricingComponent `db:"-" json:"pricing_components,omitempty"`
}

// CalculateCost calculates the cost for a given token usage
func (m *Model) CalculateCost(usageRecord UsageRecord) float64 {
	cost := 0.0

	return cost
}
