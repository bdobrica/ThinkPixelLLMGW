package providers
package providers

import (
	"testing"
	"time"
)

func TestNewOpenAIProvider_UsesConfiguredRequestTimeout(t *testing.T) {
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID:   "openai-test",
		Name: "OpenAI Test",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "test-key",
		},
		Config: map[string]any{
			"request_timeout": "15s",
		},
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed: %v", err)
	}

	op, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider type")
	}

	if op.client.Timeout != 15*time.Second {
		t.Fatalf("expected timeout 15s, got %v", op.client.Timeout)
	}
}

func TestNewOpenAIProvider_UsesDefaultTimeoutWhenUnset(t *testing.T) {
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID:   "openai-test",
		Name: "OpenAI Test",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "test-key",
		},
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed: %v", err)
	}

	op, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider type")
	}

	if op.client.Timeout != openAITimeout {
		t.Fatalf("expected default timeout %v, got %v", openAITimeout, op.client.Timeout)
	}
}

func TestNewOpenAIProvider_IgnoresInvalidRequestTimeout(t *testing.T) {
	provider, err := NewOpenAIProvider(ProviderConfig{
		ID:   "openai-test",
		Name: "OpenAI Test",
		Type: "openai",
		Credentials: map[string]string{
			"api_key": "test-key",
		},
		Config: map[string]any{
			"request_timeout": "not-a-duration",
		},
	})
	if err != nil {
		t.Fatalf("expected provider creation to succeed: %v", err)
	}

	op, ok := provider.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected OpenAIProvider type")
	}

	if op.client.Timeout != openAITimeout {
		t.Fatalf("expected default timeout %v, got %v", openAITimeout, op.client.Timeout)
	}
}
