package httpapi

import "testing"

func TestEnsureStreamUsageInPayload(t *testing.T) {
	payload := map[string]any{"stream": true}

	ensureStreamUsageInPayload(payload)

	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("expected stream_options map")
	}
	if includeUsage, ok := streamOptions["include_usage"].(bool); !ok || !includeUsage {
		t.Fatalf("expected include_usage=true, got %v", streamOptions["include_usage"])
	}
}

func TestExtractStreamUsageFromEvent_OpenAIFields(t *testing.T) {
	event := []byte(`{"usage":{"prompt_tokens":120,"completion_tokens":45,"prompt_tokens_details":{"cached_tokens":20},"completion_tokens_details":{"reasoning_tokens":5}}}`)

	usage, ok := extractStreamUsageFromEvent(event)
	if !ok {
		t.Fatalf("expected usage to be extracted")
	}

	if usage.InputTokens != 120 {
		t.Fatalf("expected input tokens 120, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 45 {
		t.Fatalf("expected output tokens 45, got %d", usage.OutputTokens)
	}
	if usage.CachedTokens != 20 {
		t.Fatalf("expected cached tokens 20, got %d", usage.CachedTokens)
	}
	if usage.ReasoningTokens != 5 {
		t.Fatalf("expected reasoning tokens 5, got %d", usage.ReasoningTokens)
	}
}

func TestExtractStreamUsageFromEvent_InputOutputFields(t *testing.T) {
	event := []byte(`{"usage":{"input_tokens":100,"output_tokens":40,"input_tokens_details":{"cached_tokens":10},"output_tokens_details":{"reasoning_tokens":3}}}`)

	usage, ok := extractStreamUsageFromEvent(event)
	if !ok {
		t.Fatalf("expected usage to be extracted")
	}

	if usage.InputTokens != 100 {
		t.Fatalf("expected input tokens 100, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 40 {
		t.Fatalf("expected output tokens 40, got %d", usage.OutputTokens)
	}
	if usage.CachedTokens != 10 {
		t.Fatalf("expected cached tokens 10, got %d", usage.CachedTokens)
	}
	if usage.ReasoningTokens != 3 {
		t.Fatalf("expected reasoning tokens 3, got %d", usage.ReasoningTokens)
	}
}

func TestExtractStreamUsageFromEvent_NoUsage(t *testing.T) {
	event := []byte(`{"id":"abc","choices":[{"delta":{"content":"hello"}}]}`)

	_, ok := extractStreamUsageFromEvent(event)
	if ok {
		t.Fatalf("expected no usage extraction")
	}
}

func TestFallbackCostFromUsage(t *testing.T) {
	cost := fallbackCostFromUsage(1000, 500)
	if cost <= 0 {
		t.Fatalf("expected positive cost, got %f", cost)
	}
}
