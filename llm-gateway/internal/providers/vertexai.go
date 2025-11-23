package providers

import (
	"context"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/models"
)

type VertexAIProvider struct {
	id string
	// TODO: project/location, credentials, etc.
}

func NewVertexAIProvider(id string) *VertexAIProvider {
	return &VertexAIProvider{id: id}
}

func (p *VertexAIProvider) ID() string {
	return p.id
}

func (p *VertexAIProvider) Type() models.ProviderType {
	return models.ProviderTypeVertexAI
}

func (p *VertexAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// TODO: implement Vertex AI call
	return &ChatResponse{
		StatusCode: 200,
		Body:       []byte(`{"message":"vertex ai provider not implemented"}`),
	}, nil
}
