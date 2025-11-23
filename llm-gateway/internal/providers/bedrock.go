package providers

import (
	"context"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/models"
)

type BedrockProvider struct {
	id string
	// TODO: AWS client/region, etc.
}

func NewBedrockProvider(id string) *BedrockProvider {
	return &BedrockProvider{id: id}
}

func (p *BedrockProvider) ID() string {
	return p.id
}

func (p *BedrockProvider) Type() models.ProviderType {
	return models.ProviderTypeBedrock
}

func (p *BedrockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// TODO: implement Bedrock call
	return &ChatResponse{
		StatusCode: 200,
		Body:       []byte(`{"message":"bedrock provider not implemented"}`),
	}, nil
}
