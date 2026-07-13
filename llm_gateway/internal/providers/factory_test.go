package providers

import (
	"slices"
	"testing"
)

func TestDefaultFactoryRegistersImplementedProviders(t *testing.T) {
	want := []string{"bedrock", "openai", "vertexai"}
	if got := NewProviderFactory().SupportedTypes(); !slices.Equal(got, want) {
		t.Fatalf("SupportedTypes() = %v, want %v", got, want)
	}
}
