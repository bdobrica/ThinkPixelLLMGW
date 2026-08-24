package responses

import "strconv"

type SupportMode string

const (
	SupportNative      SupportMode = "native"
	SupportTranslated  SupportMode = "translated"
	SupportGateway     SupportMode = "gateway"
	SupportUnavailable SupportMode = "unavailable"
)

// Capabilities is the executable provider gate for the pinned contract. An
// unavailable value must be rejected before any billable provider request.
type Capabilities struct {
	Provider              string      `json:"provider"`
	ResponsesEnabled      bool        `json:"responses_enabled"`
	ResponsesTransport    SupportMode `json:"responses_transport"`
	StateStorage          SupportMode `json:"state_storage"`
	ReasoningItems        SupportMode `json:"reasoning_items"`
	FunctionTools         SupportMode `json:"function_tools"`
	ParallelFunctionCalls SupportMode `json:"parallel_function_calls"`
	WebSearch             SupportMode `json:"web_search"`
	FileSearch            SupportMode `json:"file_search"`
	CodeInterpreter       SupportMode `json:"code_interpreter"`
	StreamingEvents       SupportMode `json:"streaming_events"`
}

var providerCapabilities = map[string]Capabilities{
	"openai": {
		Provider: "openai", ResponsesEnabled: true, ResponsesTransport: SupportNative, StateStorage: SupportNative,
		ReasoningItems: SupportNative, FunctionTools: SupportNative, ParallelFunctionCalls: SupportNative,
		WebSearch: SupportUnavailable, FileSearch: SupportUnavailable, CodeInterpreter: SupportUnavailable,
		StreamingEvents: SupportNative,
	},
	"vertexai": {
		Provider: "vertexai", ResponsesTransport: SupportTranslated, StateStorage: SupportGateway,
		ReasoningItems: SupportUnavailable, FunctionTools: SupportTranslated, ParallelFunctionCalls: SupportTranslated,
		WebSearch: SupportUnavailable, FileSearch: SupportUnavailable, CodeInterpreter: SupportUnavailable,
		StreamingEvents: SupportTranslated,
	},
	"bedrock": {
		Provider: "bedrock", ResponsesTransport: SupportTranslated, StateStorage: SupportGateway,
		ReasoningItems: SupportUnavailable, FunctionTools: SupportUnavailable, ParallelFunctionCalls: SupportUnavailable,
		WebSearch: SupportUnavailable, FileSearch: SupportUnavailable, CodeInterpreter: SupportUnavailable,
		StreamingEvents: SupportTranslated,
	},
}

func ProviderCapabilities(providerType string) (Capabilities, bool) {
	capabilities, ok := providerCapabilities[providerType]
	return capabilities, ok
}

// ModelCapabilities combines catalog flags with provider protocol support.
// It avoids claiming a feature merely because one side of the route supports it.
type ModelCapabilities struct {
	Responses             bool
	StateStorage          bool
	Reasoning             bool
	FunctionTools         bool
	ParallelFunctionCalls bool
	Streaming             bool
	WebSearch             bool
	FileSearch            bool
	CodeInterpreter       bool
}

func ResolveModelCapabilities(provider Capabilities, supportsReasoning, supportsFunctions, supportsParallel, supportsStreaming, supportsWebSearch bool) ModelCapabilities {
	available := func(mode SupportMode) bool { return mode != SupportUnavailable }
	return ModelCapabilities{
		Responses:             provider.ResponsesEnabled && available(provider.ResponsesTransport),
		StateStorage:          provider.ResponsesEnabled && available(provider.StateStorage),
		Reasoning:             provider.ResponsesEnabled && supportsReasoning && available(provider.ReasoningItems),
		FunctionTools:         provider.ResponsesEnabled && supportsFunctions && available(provider.FunctionTools),
		ParallelFunctionCalls: provider.ResponsesEnabled && supportsFunctions && supportsParallel && available(provider.ParallelFunctionCalls),
		Streaming:             provider.ResponsesEnabled && supportsStreaming && available(provider.StreamingEvents),
		WebSearch:             provider.ResponsesEnabled && supportsWebSearch && available(provider.WebSearch),
		FileSearch:            provider.ResponsesEnabled && available(provider.FileSearch),
		CodeInterpreter:       provider.ResponsesEnabled && available(provider.CodeInterpreter),
	}
}

// ValidateCapabilities rejects an unsupported request before provider work.
// Model catalog flags must already have been intersected through
// ResolveModelCapabilities.
func ValidateCapabilities(request CreateRequest, capabilities ModelCapabilities) error {
	if !capabilities.Responses {
		return invalid("model", "unsupported_endpoint", "the selected model does not support the Responses API")
	}
	if request.PreviousResponseID != "" && !capabilities.StateStorage {
		return invalid("previous_response_id", "unsupported_parameter", "the selected route does not support response state")
	}
	if request.Reasoning != nil && !capabilities.Reasoning {
		return invalid("reasoning", "unsupported_parameter", "the selected model does not support reasoning items")
	}
	if request.Stream && !capabilities.Streaming {
		return invalid("stream", "unsupported_parameter", "the selected model does not support Responses streaming")
	}
	if request.ParallelToolCalls != nil && *request.ParallelToolCalls && !capabilities.ParallelFunctionCalls {
		return invalid("parallel_tool_calls", "unsupported_parameter", "the selected model does not support parallel function calls")
	}
	for index, tool := range request.Tools {
		param := "tools[" + strconv.Itoa(index) + "]"
		switch tool.Type {
		case "function":
			if !capabilities.FunctionTools {
				return invalid(param, "unsupported_tool", "the selected model does not support function tools")
			}
		case "web_search", "web_search_preview":
			if !capabilities.WebSearch {
				return invalid(param, "unsupported_tool", "web search is not enabled for the selected route")
			}
		case "file_search":
			if !capabilities.FileSearch {
				return invalid(param, "unsupported_tool", "file search is not enabled for the selected route")
			}
		case "code_interpreter":
			if !capabilities.CodeInterpreter {
				return invalid(param, "unsupported_tool", "code interpreter is not enabled for the selected route")
			}
		}
	}
	return nil
}
