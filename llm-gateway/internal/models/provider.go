package models

import "time"

// ProviderType enumerates supported provider types.
type ProviderType string

const (
	ProviderTypeOpenAI   ProviderType = "openai"
	ProviderTypeVertexAI ProviderType = "vertexai"
	ProviderTypeBedrock  ProviderType = "bedrock"
)

// Provider represents an LLM provider configuration.
type Provider struct {
	ID        string
	Name      string
	Type      ProviderType
	FromFile  bool // true if credentials/config come from a mounted file
	Active    bool
	CreatedAt time.Time
}
