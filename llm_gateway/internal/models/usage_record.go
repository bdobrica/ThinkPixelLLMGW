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
