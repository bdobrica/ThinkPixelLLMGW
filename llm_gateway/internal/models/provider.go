package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ProviderType enumerates supported provider types.
type ProviderType string

const (
	ProviderTypeOpenAI   ProviderType = "openai"
	ProviderTypeVertexAI ProviderType = "vertexai"
	ProviderTypeBedrock  ProviderType = "bedrock"
)

// Provider represents an LLM provider configuration
type Provider struct {
	ID                   uuid.UUID       `db:"id"`
	Name                 string          `db:"name"`
	DisplayName          string          `db:"display_name"`
	ProviderType         string          `db:"provider_type"`
	EncryptedCredentials json.RawMessage `db:"encrypted_credentials"`
	Config               json.RawMessage `db:"config"`
	Enabled              bool            `db:"enabled"`
	CreatedAt            time.Time       `db:"created_at"`
	UpdatedAt            time.Time       `db:"updated_at"`
}
