package providers

import (
	"context"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/models"
)

type OpenAIProvider struct {
	id string
	// TODO: add HTTP client, base URL, API key/credentials, etc.
}

func NewOpenAIProvider(id string) *OpenAIProvider {
	return &OpenAIProvider{id: id}
}

func (p *OpenAIProvider) ID() string {
	return p.id
}

func (p *OpenAIProvider) Type() models.ProviderType {
	return models.ProviderTypeOpenAI
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// TODO: implement actual OpenAI Chat API call
	return &ChatResponse{
		StatusCode: 200,
		Body:       []byte(`{"message":"openai provider not implemented"}`),
	}, nil
}
