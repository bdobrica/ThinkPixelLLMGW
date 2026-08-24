package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	maxMetadataEntries        = 16
	maxMetadataKeyLen         = 64
	maxMetadataValueLen       = 512
	maxFunctionDescriptionLen = 1024
	maxFunctionSchemaBytes    = 64 << 10
	maxFunctionSchemaDepth    = 16
)

var functionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

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
	if err := validateUniqueFunctionNames(r.Tools); err != nil {
		return err
	}
	if err := validateToolChoice(r.ToolChoice, r.Tools); err != nil {
		return err
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
		if !functionNamePattern.MatchString(item.Name) {
			return invalid(param+".name", "invalid_value", "function name must contain only letters, numbers, underscores, or hyphens and be at most 64 characters")
		}
		if !json.Valid([]byte(item.Arguments)) {
			return invalid(param+".arguments", "invalid_json", "function_call arguments must be valid JSON")
		}
	case "function_call_output":
		if item.CallID == "" || len(item.Output) == 0 {
			return invalid(param, "missing_required_parameter", "function_call_output requires call_id and output")
		}
		if !json.Valid(item.Output) || bytes.Equal(bytes.TrimSpace(item.Output), []byte("null")) {
			return invalid(param+".output", "invalid_value", "function_call_output output must be a JSON string or structured JSON value")
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
		if !functionNamePattern.MatchString(tool.Name) {
			return invalid(param+".name", "invalid_value", "function name must contain only letters, numbers, underscores, or hyphens and be at most 64 characters")
		}
		if len(tool.Description) > maxFunctionDescriptionLen {
			return invalid(param+".description", "invalid_value", "function description may contain at most %d characters", maxFunctionDescriptionLen)
		}
		if tool.Parameters == nil {
			return invalid(param+".parameters", "missing_required_parameter", "function tool parameters are required")
		}
		encoded, err := json.Marshal(tool.Parameters)
		if err != nil || len(encoded) > maxFunctionSchemaBytes {
			return invalid(param+".parameters", "invalid_value", "function parameters must be valid JSON Schema no larger than %d bytes", maxFunctionSchemaBytes)
		}
		if schemaDepth(tool.Parameters) > maxFunctionSchemaDepth {
			return invalid(param+".parameters", "invalid_value", "function parameters may be nested at most %d levels", maxFunctionSchemaDepth)
		}
		if schemaType, ok := tool.Parameters["type"].(string); !ok || schemaType != "object" {
			return invalid(param+".parameters.type", "invalid_value", "function parameters must use a top-level object schema")
		}
		if tool.Strict != nil && *tool.Strict {
			if err := validateStrictSchema(tool.Parameters, param+".parameters"); err != nil {
				return err
			}
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

func validateUniqueFunctionNames(tools []Tool) error {
	seen := make(map[string]struct{}, len(tools))
	for index, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		if _, exists := seen[tool.Name]; exists {
			return invalid(fmt.Sprintf("tools[%d].name", index), "invalid_value", "function tool names must be unique")
		}
		seen[tool.Name] = struct{}{}
	}
	return nil
}

func validateToolChoice(raw json.RawMessage, tools []Tool) error {
	if len(raw) == 0 {
		return nil
	}
	available := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool.Type == "function" {
			available[tool.Name] = struct{}{}
		}
	}
	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case "none", "auto":
			return nil
		case "required":
			if len(tools) == 0 {
				return invalid("tool_choice", "invalid_parameter_combination", "tool_choice required requires at least one tool")
			}
			return nil
		default:
			return invalid("tool_choice", "invalid_value", "tool_choice must be none, auto, required, a named function, or allowed_tools")
		}
	}
	var choice struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Mode  string `json:"mode"`
		Tools []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&choice); err != nil {
		return invalid("tool_choice", "invalid_value", "tool_choice has an invalid shape")
	}
	switch choice.Type {
	case "function":
		if choice.Name == "" {
			return invalid("tool_choice.name", "missing_required_parameter", "named function tool_choice requires name")
		}
		if _, ok := available[choice.Name]; !ok {
			return invalid("tool_choice.name", "invalid_value", "tool_choice references an unavailable function")
		}
	case "allowed_tools":
		if choice.Mode != "auto" && choice.Mode != "required" {
			return invalid("tool_choice.mode", "invalid_value", "allowed_tools mode must be auto or required")
		}
		if len(choice.Tools) == 0 {
			return invalid("tool_choice.tools", "missing_required_parameter", "allowed_tools requires at least one tool")
		}
		seen := map[string]struct{}{}
		for index, tool := range choice.Tools {
			if tool.Type != "function" {
				return invalid(fmt.Sprintf("tool_choice.tools[%d].type", index), "unsupported_tool", "only function tools are enabled")
			}
			if _, ok := available[tool.Name]; !ok {
				return invalid(fmt.Sprintf("tool_choice.tools[%d].name", index), "invalid_value", "allowed_tools references an unavailable function")
			}
			if _, duplicate := seen[tool.Name]; duplicate {
				return invalid(fmt.Sprintf("tool_choice.tools[%d].name", index), "invalid_value", "allowed_tools entries must be unique")
			}
			seen[tool.Name] = struct{}{}
		}
	default:
		return invalid("tool_choice.type", "invalid_value", "unsupported tool_choice type")
	}
	return nil
}

func schemaDepth(value any) int {
	maxChild := 0
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if depth := schemaDepth(child); depth > maxChild {
				maxChild = depth
			}
		}
	case []any:
		for _, child := range typed {
			if depth := schemaDepth(child); depth > maxChild {
				maxChild = depth
			}
		}
	default:
		return 0
	}
	return maxChild + 1
}

func validateStrictSchema(schema map[string]any, param string) error {
	if schemaType, _ := schema["type"].(string); schemaType == "object" {
		additional, ok := schema["additionalProperties"].(bool)
		if !ok || additional {
			return invalid(param+".additionalProperties", "invalid_json_schema", "strict object schemas require additionalProperties=false")
		}
		properties, _ := schema["properties"].(map[string]any)
		requiredValues, _ := schema["required"].([]any)
		required := make(map[string]struct{}, len(requiredValues))
		for _, value := range requiredValues {
			name, ok := value.(string)
			if !ok {
				return invalid(param+".required", "invalid_json_schema", "strict schema required entries must be strings")
			}
			required[name] = struct{}{}
		}
		for name, child := range properties {
			if _, ok := required[name]; !ok {
				return invalid(param+".required", "invalid_json_schema", "strict schemas must require every property")
			}
			if childSchema, ok := child.(map[string]any); ok {
				if err := validateStrictSchema(childSchema, param+".properties."+name); err != nil {
					return err
				}
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		return validateStrictSchema(items, param+".items")
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
