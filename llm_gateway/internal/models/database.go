package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Model represents an LLM model from the BerriAI catalog
type Model struct {
	ID              uuid.UUID `db:"id"`
	ModelName       string    `db:"model_name"`
	LiteLLMProvider string    `db:"litellm_provider"`

	// Pricing (all in USD per token unless specified)
	InputCostPerToken           *float64 `db:"input_cost_per_token"`
	OutputCostPerToken          *float64 `db:"output_cost_per_token"`
	InputCostPerTokenBatches    *float64 `db:"input_cost_per_token_batches"`
	OutputCostPerTokenBatches   *float64 `db:"output_cost_per_token_batches"`
	CacheReadInputTokenCost     *float64 `db:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost *float64 `db:"cache_creation_input_token_cost"`
	InputCostPerAudioToken      *float64 `db:"input_cost_per_audio_token"`
	OutputCostPerAudioToken     *float64 `db:"output_cost_per_audio_token"`
	OutputCostPerReasoningToken *float64 `db:"output_cost_per_reasoning_token"`
	InputCostPerTokenAbove128K  *float64 `db:"input_cost_per_token_above_128k_tokens"`
	OutputCostPerTokenAbove128K *float64 `db:"output_cost_per_token_above_128k_tokens"`
	InputCostPerTokenAbove200K  *float64 `db:"input_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove200K *float64 `db:"output_cost_per_token_above_200k_tokens"`
	InputCostPerImage           *float64 `db:"input_cost_per_image"`
	OutputCostPerImage          *float64 `db:"output_cost_per_image"`
	InputCostPerCharacter       *float64 `db:"input_cost_per_character"`
	OutputCostPerCharacter      *float64 `db:"output_cost_per_character"`
	InputCostPerQuery           *float64 `db:"input_cost_per_query"`

	// Context windows
	MaxInputTokens  *int `db:"max_input_tokens"`
	MaxOutputTokens *int `db:"max_output_tokens"`
	MaxTokens       *int `db:"max_tokens"`

	// Mode (chat, completion, embedding, etc.)
	Mode string `db:"mode"`

	// Feature flags
	SupportsFunctionCalling         bool `db:"supports_function_calling"`
	SupportsParallelFunctionCalling bool `db:"supports_parallel_function_calling"`
	SupportsToolChoice              bool `db:"supports_tool_choice"`
	SupportsVision                  bool `db:"supports_vision"`
	SupportsAudioInput              bool `db:"supports_audio_input"`
	SupportsAudioOutput             bool `db:"supports_audio_output"`
	SupportsPromptCaching           bool `db:"supports_prompt_caching"`
	SupportsReasoning               bool `db:"supports_reasoning"`
	SupportsResponseSchema          bool `db:"supports_response_schema"`
	SupportsSystemMessages          bool `db:"supports_system_messages"`
	SupportsNativeStreaming         bool `db:"supports_native_streaming"`
	SupportsPDFInput                bool `db:"supports_pdf_input"`
	SupportsWebSearch               bool `db:"supports_web_search"`

	// Arrays
	SupportedEndpoints        pq.StringArray `db:"supported_endpoints"`
	SupportedModalities       pq.StringArray `db:"supported_modalities"`
	SupportedOutputModalities pq.StringArray `db:"supported_output_modalities"`

	// Full BerriAI metadata (JSONB)
	Metadata json.RawMessage `db:"metadata"`

	// Sync tracking
	SyncSource   string     `db:"sync_source"`
	SyncVersion  *string    `db:"sync_version"`
	LastSyncedAt *time.Time `db:"last_synced_at"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// CalculateCost calculates the cost for a given token usage
func (m *Model) CalculateCost(inputTokens, outputTokens, cachedTokens, reasoningTokens int) float64 {
	cost := 0.0

	if m.InputCostPerToken != nil {
		cost += float64(inputTokens) * *m.InputCostPerToken
	}

	if m.OutputCostPerToken != nil {
		cost += float64(outputTokens) * *m.OutputCostPerToken
	}

	if m.CacheReadInputTokenCost != nil && cachedTokens > 0 {
		cost += float64(cachedTokens) * *m.CacheReadInputTokenCost
	}

	if m.OutputCostPerReasoningToken != nil && reasoningTokens > 0 {
		cost += float64(reasoningTokens) * *m.OutputCostPerReasoningToken
	}

	return cost
}

// KeyMetadata represents flexible metadata for an API key
type KeyMetadata struct {
	ID           uuid.UUID `db:"id"`
	APIKeyID     uuid.UUID `db:"api_key_id"`
	MetadataType string    `db:"metadata_type"` // tag, label, custom_field
	Key          string    `db:"key"`
	Value        string    `db:"value"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// UsageRecord represents a single API request audit log
type UsageRecord struct {
	ID              uuid.UUID       `db:"id"`
	APIKeyID        uuid.UUID       `db:"api_key_id"`
	ModelID         *uuid.UUID      `db:"model_id"`
	ProviderID      *uuid.UUID      `db:"provider_id"`
	RequestID       *string         `db:"request_id"`
	ModelName       string          `db:"model_name"`
	Endpoint        *string         `db:"endpoint"`
	InputTokens     int             `db:"input_tokens"`
	OutputTokens    int             `db:"output_tokens"`
	CachedTokens    *int            `db:"cached_tokens"`
	ReasoningTokens *int            `db:"reasoning_tokens"`
	InputCostUSD    float64         `db:"input_cost_usd"`
	OutputCostUSD   float64         `db:"output_cost_usd"`
	CacheCostUSD    *float64        `db:"cache_cost_usd"`
	TotalCostUSD    float64         `db:"total_cost_usd"`
	ResponseTimeMS  *int            `db:"response_time_ms"`
	StatusCode      *int            `db:"status_code"`
	ErrorMessage    *string         `db:"error_message"`
	Metadata        json.RawMessage `db:"metadata"`
	CreatedAt       time.Time       `db:"created_at"`
}

// MonthlyUsageSummary represents pre-aggregated monthly usage
type MonthlyUsageSummary struct {
	ID                uuid.UUID  `db:"id"`
	APIKeyID          uuid.UUID  `db:"api_key_id"`
	Year              int        `db:"year"`
	Month             int        `db:"month"`
	TotalRequests     int64      `db:"total_requests"`
	TotalInputTokens  int64      `db:"total_input_tokens"`
	TotalOutputTokens int64      `db:"total_output_tokens"`
	TotalCostUSD      float64    `db:"total_cost_usd"`
	LastRequestAt     *time.Time `db:"last_request_at"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}
