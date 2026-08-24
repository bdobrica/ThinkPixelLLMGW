package responses

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestStreamEventGoldenSequence(t *testing.T) {
	file, err := os.Open("testdata/stream_events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	want := []EventType{EventResponseCreated, EventOutputItemAdded, EventContentPartAdded, EventOutputTextDelta, EventOutputTextDone, EventResponseCompleted}
	scanner := bufio.NewScanner(file)
	index := 0
	for scanner.Scan() {
		event, err := DecodeStreamEvent(scanner.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if index >= len(want) || event.EventType() != want[index] || event.SequenceNumber() != int64(index) {
			t.Fatalf("event %d = type %q sequence %d", index, event.EventType(), event.SequenceNumber())
		}
		index++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(want) {
		t.Fatalf("decoded %d events, want %d", index, len(want))
	}
}

func TestDecodeCreateRequestGolden(t *testing.T) {
	body, err := os.ReadFile("testdata/create_request.json")
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeCreateRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if request.Model != "gpt-5.1" || request.Input.String != nil || len(request.Input.Items) != 2 {
		t.Fatalf("unexpected request decode: %#v", request)
	}
	if request.Input.Items[0].Type != "message" || request.Input.Items[1].Type != "function_call_output" {
		t.Fatalf("item order or type changed: %#v", request.Input.Items)
	}
	if len(request.Tools) != 1 || request.Tools[0].Type != "function" || request.Tools[0].Name != "get_weather" {
		t.Fatalf("function tool changed: %#v", request.Tools)
	}
	if request.Truncation != TruncationDisabled || len(request.Include) != 1 || request.Include[0] != IncludeEncryptedReasoning {
		t.Fatalf("request controls changed: %#v", request)
	}
}

func TestResponseGoldenPreservesHeterogeneousOutputOrderAndUsage(t *testing.T) {
	body, err := os.ReadFile("testdata/response.json")
	if err != nil {
		t.Fatal(err)
	}
	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != "resp_contract_1" || response.Object != "response" || response.Status != StatusCompleted {
		t.Fatalf("response envelope changed: %#v", response)
	}
	wantTypes := []string{"reasoning", "message", "function_call"}
	if len(response.Output) != len(wantTypes) {
		t.Fatalf("got %d output items", len(response.Output))
	}
	for index, want := range wantTypes {
		if response.Output[index].Type != want {
			t.Fatalf("output[%d] type = %q, want %q", index, response.Output[index].Type, want)
		}
	}
	if response.Usage == nil || response.Usage.InputTokensDetails.CachedTokens != 5 || response.Usage.OutputTokensDetails.ReasoningTokens != 7 || response.Usage.TotalTokens != 55 {
		t.Fatalf("usage detail changed: %#v", response.Usage)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Response
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	for index := range wantTypes {
		if roundTrip.Output[index].ID != response.Output[index].ID || roundTrip.Output[index].Type != response.Output[index].Type {
			t.Fatalf("round trip changed output[%d]", index)
		}
	}
}

func TestStringInputUnion(t *testing.T) {
	request, err := DecodeCreateRequest([]byte(`{"model":"gpt-5.1","input":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if request.Input.String == nil || *request.Input.String != "hello" || request.Input.Items != nil {
		t.Fatalf("unexpected string input: %#v", request.Input)
	}
}

func TestCreateRequestValidation(t *testing.T) {
	tests := []struct {
		name, body, param, code string
	}{
		{"missing model", `{"input":"hello"}`, "model", "missing_required_parameter"},
		{"missing input", `{"model":"gpt-5.1"}`, "input", "missing_required_parameter"},
		{"unknown field", `{"model":"gpt-5.1","input":"hello","mystery":true}`, "", "invalid_json"},
		{"state conflict", `{"model":"gpt-5.1","input":"hello","previous_response_id":"resp_1","conversation":"conv_1"}`, "previous_response_id", "invalid_parameter_combination"},
		{"stream options", `{"model":"gpt-5.1","input":"hello","stream_options":{"include_obfuscation":false}}`, "stream_options", "invalid_parameter_combination"},
		{"background stream", `{"model":"gpt-5.1","input":"hello","background":true,"stream":true}`, "background", "unsupported_parameter_combination"},
		{"tool", `{"model":"gpt-5.1","input":"hello","tools":[{"type":"function","name":"weather"}]}`, "tools[0].parameters", "missing_required_parameter"},
		{"tool name", `{"model":"gpt-5.1","input":"hello","tools":[{"type":"function","name":"bad name","parameters":{"type":"object"}}]}`, "tools[0].name", "invalid_value"},
		{"duplicate tool name", `{"model":"gpt-5.1","input":"hello","tools":[{"type":"function","name":"weather","parameters":{"type":"object"}},{"type":"function","name":"weather","parameters":{"type":"object"}}]}`, "tools[1].name", "invalid_value"},
		{"strict schema", `{"model":"gpt-5.1","input":"hello","tools":[{"type":"function","name":"weather","strict":true,"parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]}`, "tools[0].parameters.additionalProperties", "invalid_json_schema"},
		{"named tool choice", `{"model":"gpt-5.1","input":"hello","tools":[{"type":"function","name":"weather","parameters":{"type":"object"}}],"tool_choice":{"type":"function","name":"missing"}}`, "tool_choice.name", "invalid_value"},
		{"allowed tools", `{"model":"gpt-5.1","input":"hello","tools":[{"type":"function","name":"weather","parameters":{"type":"object"}}],"tool_choice":{"type":"allowed_tools","mode":"sometimes","tools":[{"type":"function","name":"weather"}]}}`, "tool_choice.mode", "invalid_value"},
		{"invalid call arguments", `{"model":"gpt-5.1","input":[{"type":"function_call","call_id":"call_1","name":"weather","arguments":"{"}]}`, "input[0].arguments", "invalid_json"},
		{"input item", `{"model":"gpt-5.1","input":[{"type":"message","role":"tool","content":"x"}]}`, "input[0].role", "invalid_value"},
		{"include", `{"model":"gpt-5.1","input":"hello","include":["unknown"]}`, "include[0]", "invalid_value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeCreateRequest([]byte(test.body))
			var validationError *ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("got %v, want ValidationError", err)
			}
			if validationError.Param != test.param || validationError.Code != test.code {
				t.Fatalf("got param=%q code=%q, want param=%q code=%q", validationError.Param, validationError.Code, test.param, test.code)
			}
			envelope := validationError.Envelope()
			if envelope.Error.Type != "invalid_request_error" || envelope.Error.Param == nil || *envelope.Error.Param != test.param {
				t.Fatalf("unexpected error envelope: %#v", envelope)
			}
		})
	}
}

func TestMetadataLimits(t *testing.T) {
	longKey := strings.Repeat("k", 65)
	request := CreateRequest{Model: "gpt-5.1", Input: Input{Set: true, String: stringPointer("hello")}, Metadata: map[string]string{longKey: "value"}}
	var validationError *ValidationError
	if err := request.Validate(); !errors.As(err, &validationError) || validationError.Code != "metadata_key_too_long" {
		t.Fatalf("got %v", err)
	}
}

func TestProviderCapabilityMatrix(t *testing.T) {
	openAI, ok := ProviderCapabilities("openai")
	if !ok || openAI.ResponsesTransport != SupportNative || openAI.ReasoningItems != SupportNative {
		t.Fatalf("unexpected OpenAI capabilities: %#v", openAI)
	}
	if openAI.CodeInterpreter != SupportUnavailable {
		t.Fatalf("hosted tools must remain unavailable until Step 24: %#v", openAI)
	}
	bedrock, ok := ProviderCapabilities("bedrock")
	if !ok || bedrock.ResponsesTransport != SupportTranslated || bedrock.FunctionTools != SupportUnavailable {
		t.Fatalf("unexpected Bedrock capabilities: %#v", bedrock)
	}
	if _, ok := ProviderCapabilities("unknown"); ok {
		t.Fatal("unknown provider unexpectedly has capabilities")
	}

	model := ResolveModelCapabilities(openAI, true, true, true, true, true)
	if !model.Responses || !model.Reasoning || !model.FunctionTools || !model.ParallelFunctionCalls || !model.Streaming {
		t.Fatalf("unexpected resolved model capabilities: %#v", model)
	}
	if model.WebSearch {
		t.Fatal("model catalog flag must not override unavailable provider feature")
	}
}

func TestCapabilityValidationRejectsBeforeProviderExecution(t *testing.T) {
	request, err := DecodeCreateRequest([]byte(`{"model":"gpt-5.1","input":"hello","tools":[{"type":"web_search"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	openAI, _ := ProviderCapabilities("openai")
	openAI.ResponsesEnabled = true
	capabilities := ResolveModelCapabilities(openAI, true, true, true, true, true)
	err = ValidateCapabilities(*request, capabilities)
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Param != "tools[0]" || validationError.Code != "unsupported_tool" {
		t.Fatalf("got %v", err)
	}
}

func stringPointer(value string) *string { return &value }
