package httpapi

import (
	"testing"

	"github.com/google/uuid"
)

func TestEnsureStreamUsageInPayload(t *testing.T) {
	payload := map[string]any{
		"stream": true,
		"stream_options": map[string]any{
			"include_obfuscation": false,
		},
	}

	ensureStreamUsageInPayload(payload)

	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("expected stream_options map")
	}
	if includeUsage, ok := streamOptions["include_usage"].(bool); !ok || !includeUsage {
		t.Fatalf("expected include_usage=true, got %v", streamOptions["include_usage"])
	}
	if includeObfuscation, ok := streamOptions["include_obfuscation"].(bool); !ok || includeObfuscation {
		t.Fatalf("compatible stream option was overwritten: %v", streamOptions)
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

func TestParseUsageRecordIDs_Valid(t *testing.T) {
	apiKeyID := uuid.New().String()
	requestID := uuid.New().String()

	apiKeyUUID, requestUUID, err := parseUsageRecordIDs(apiKeyID, requestID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if apiKeyUUID.String() != apiKeyID {
		t.Fatalf("expected api key UUID %s, got %s", apiKeyID, apiKeyUUID.String())
	}
	if requestUUID.String() != requestID {
		t.Fatalf("expected request UUID %s, got %s", requestID, requestUUID.String())
	}
}

func TestParseUsageRecordIDs_Invalid(t *testing.T) {
	_, _, err := parseUsageRecordIDs("not-a-uuid", uuid.New().String())
	if err == nil {
		t.Fatal("expected error for invalid api key UUID")
	}

	_, _, err = parseUsageRecordIDs(uuid.New().String(), "not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid request UUID")
	}
}
