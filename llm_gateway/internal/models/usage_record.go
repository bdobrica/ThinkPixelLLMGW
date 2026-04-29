package models

import (
	"time"

	"github.com/google/uuid"
)

// UsageRecord represents a single API request audit log
type UsageRecord struct {
	ID              uuid.UUID `db:"id"`
	APIKeyID        uuid.UUID `db:"api_key_id"`
	ModelID         uuid.UUID `db:"model_id"`
	ProviderID      uuid.UUID `db:"provider_id"`
	RequestID       uuid.UUID `db:"request_id"`
	ModelName       string    `db:"model_name"`
	Endpoint        string    `db:"endpoint"`
	InputTokens     int       `db:"input_tokens"`
	OutputTokens    int       `db:"output_tokens"`
	CachedTokens    int       `db:"cached_tokens"`
	ReasoningTokens int       `db:"reasoning_tokens"`
	ResponseTimeMS  int       `db:"response_time_ms"`
	StatusCode      int       `db:"status_code"`
	ErrorMessage    string    `db:"error_message"`
	CreatedAt       time.Time `db:"created_at"`
}

// MonthlyUsageSummary stores persisted monthly usage and billing totals per API key.
type MonthlyUsageSummary struct {
	ID                   uuid.UUID `db:"id"`
	APIKeyID             uuid.UUID `db:"api_key_id"`
	Year                 int       `db:"year"`
	Month                int       `db:"month"`
	TotalRequests        int       `db:"total_requests"`
	TotalInputTokens     int       `db:"total_input_tokens"`
	TotalOutputTokens    int       `db:"total_output_tokens"`
	TotalCachedTokens    int       `db:"total_cached_tokens"`
	TotalReasoningTokens int       `db:"total_reasoning_tokens"`
	TotalCostUSD         float64   `db:"total_cost_usd"`
	CreatedAt            time.Time `db:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"`
}
