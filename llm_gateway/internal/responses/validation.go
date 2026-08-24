package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	maxMetadataEntries  = 16
	maxMetadataKeyLen   = 64
	maxMetadataValueLen = 512
)

type ValidationError struct {
	Param   string
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) Envelope() ErrorEnvelope {
	param := e.Param
	return ErrorEnvelope{Error: APIError{Message: e.Message, Type: "invalid_request_error", Param: &param, Code: e.Code}}
}

func invalid(param, code, format string, args ...any) error {
	return &ValidationError{Param: param, Code: code, Message: fmt.Sprintf(format, args...)}
}

// DecodeCreateRequest rejects unknown top-level fields to prevent silently
// accepting options that the gateway does not implement in this snapshot.
func DecodeCreateRequest(data []byte) (*CreateRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var request CreateRequest
	if err := decoder.Decode(&request); err != nil {
		return nil, invalid("", "invalid_json", "invalid Responses request: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, invalid("", "invalid_json", "request body must contain one JSON object")
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return &request, nil
}

func (r CreateRequest) Validate() error {
	if strings.TrimSpace(r.Model) == "" {
		return invalid("model", "missing_required_parameter", "missing required parameter: model")
	}
	if !r.Input.Set {
		return invalid("input", "missing_required_parameter", "missing required parameter: input")
	}
	if r.Input.String != nil && *r.Input.String == "" {
		return invalid("input", "invalid_value", "input must not be empty")
	}
	if r.Input.String == nil && len(r.Input.Items) == 0 {
		return invalid("input", "invalid_value", "input item array must not be empty")
	}
	for index, item := range r.Input.Items {
		if err := validateInputItem(index, item); err != nil {
			return err
		}
	}
	if r.PreviousResponseID != "" && r.Conversation != nil {
		return invalid("previous_response_id", "invalid_parameter_combination", "previous_response_id cannot be used with conversation")
	}
	if r.Conversation != nil && strings.TrimSpace(r.Conversation.ID) == "" {
		return invalid("conversation.id", "invalid_value", "conversation.id must not be empty")
	}
	if r.StreamOptions != nil && !r.Stream {
		return invalid("stream_options", "invalid_parameter_combination", "stream_options requires stream=true")
	}
	if r.Background && r.Stream {
		return invalid("background", "unsupported_parameter_combination", "background streaming is not supported by this gateway snapshot")
	}
	if r.Truncation != "" && r.Truncation != TruncationAuto && r.Truncation != TruncationDisabled {
		return invalid("truncation", "invalid_value", "truncation must be auto or disabled")
	}
	if r.MaxOutputTokens != nil && *r.MaxOutputTokens <= 0 {
		return invalid("max_output_tokens", "invalid_value", "max_output_tokens must be greater than zero")
	}
	if r.MaxToolCalls != nil && *r.MaxToolCalls <= 0 {
		return invalid("max_tool_calls", "invalid_value", "max_tool_calls must be greater than zero")
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return invalid("temperature", "invalid_value", "temperature must be between 0 and 2")
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return invalid("top_p", "invalid_value", "top_p must be between 0 and 1")
	}
	if r.TopLogprobs != nil && (*r.TopLogprobs < 0 || *r.TopLogprobs > 20) {
		return invalid("top_logprobs", "invalid_value", "top_logprobs must be between 0 and 20")
	}
	if len(r.Metadata) > maxMetadataEntries {
		return invalid("metadata", "metadata_too_large", "metadata may contain at most %d entries", maxMetadataEntries)
	}
	for key, value := range r.Metadata {
		if len(key) > maxMetadataKeyLen {
			return invalid("metadata", "metadata_key_too_long", "metadata keys may contain at most %d characters", maxMetadataKeyLen)
		}
		if len(value) > maxMetadataValueLen {
			return invalid("metadata."+key, "metadata_value_too_long", "metadata values may contain at most %d characters", maxMetadataValueLen)
		}
	}
	for index, tool := range r.Tools {
		if err := validateTool(index, tool); err != nil {
			return err
		}
	}
	for index, include := range r.Include {
		if !validInclude(include) {
			return invalid(fmt.Sprintf("include[%d]", index), "invalid_value", "unsupported include value %q", include)
		}
	}
	return nil
}

func validateInputItem(index int, item InputItem) error {
	param := fmt.Sprintf("input[%d]", index)
	switch item.Type {
	case "", "message":
		if item.Role != "user" && item.Role != "assistant" && item.Role != "system" && item.Role != "developer" {
			return invalid(param+".role", "invalid_value", "message role must be user, assistant, system, or developer")
		}
		if len(item.Content) == 0 || bytes.Equal(item.Content, []byte("null")) {
			return invalid(param+".content", "missing_required_parameter", "message content is required")
		}
		if err := validateMessageContent(param+".content", item.Content); err != nil {
			return err
		}
	case "function_call":
		if item.CallID == "" || item.Name == "" || item.Arguments == "" {
			return invalid(param, "missing_required_parameter", "function_call requires call_id, name, and arguments")
		}
	case "function_call_output":
		if item.CallID == "" || len(item.Output) == 0 {
			return invalid(param, "missing_required_parameter", "function_call_output requires call_id and output")
		}
	case "reasoning":
		// Summary/encrypted content can both be omitted for provider-managed state.
	default:
		return invalid(param+".type", "unsupported_item_type", "unsupported input item type %q", item.Type)
	}
	return nil
}

func validateMessageContent(param string, content json.RawMessage) error {
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		if text == "" {
			return invalid(param, "invalid_value", "message content must not be empty")
		}
		return nil
	}
	var parts []InputContentPart
	if err := json.Unmarshal(content, &parts); err != nil || len(parts) == 0 {
		return invalid(param, "invalid_union", "message content must be a non-empty string or content-part array")
	}
	for index, part := range parts {
		partParam := fmt.Sprintf("%s[%d]", param, index)
		switch part.Type {
		case "input_text":
			if part.Text == "" {
				return invalid(partParam+".text", "missing_required_parameter", "input_text requires text")
			}
		case "input_image":
			if part.ImageURL == "" && part.FileID == "" {
				return invalid(partParam, "missing_required_parameter", "input_image requires image_url or file_id")
			}
		case "input_file":
			if part.FileID == "" && part.FileData == "" {
				return invalid(partParam, "missing_required_parameter", "input_file requires file_id or file_data")
			}
		case "output_text", "refusal":
			// Accepted when replaying prior assistant output as input.
		default:
			return invalid(partParam+".type", "unsupported_content_type", "unsupported content part type %q", part.Type)
		}
	}
	return nil
}

func validateTool(index int, tool Tool) error {
	param := fmt.Sprintf("tools[%d]", index)
	switch tool.Type {
	case "function":
		if strings.TrimSpace(tool.Name) == "" {
			return invalid(param+".name", "missing_required_parameter", "function tool name is required")
		}
		if tool.Parameters == nil {
			return invalid(param+".parameters", "missing_required_parameter", "function tool parameters are required")
		}
	case "web_search", "web_search_preview", "code_interpreter":
	case "file_search":
		if len(tool.VectorStoreIDs) == 0 {
			return invalid(param+".vector_store_ids", "missing_required_parameter", "file_search requires vector_store_ids")
		}
	default:
		return invalid(param+".type", "unsupported_tool", "unsupported tool type %q in contract snapshot %s", tool.Type, ContractSnapshot)
	}
	return nil
}

func validInclude(value Include) bool {
	switch value {
	case IncludeWebSearchSources, IncludeCodeInterpreterOutputs, IncludeComputerImageURL,
		IncludeFileSearchResults, IncludeInputImageURL, IncludeOutputTextLogprobs, IncludeEncryptedReasoning:
		return true
	default:
		return false
	}
}
