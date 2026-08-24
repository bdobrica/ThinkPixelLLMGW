// Package responses defines the gateway's pinned OpenAI Responses API wire contract.
//
// Contract snapshot: 2026-08-24.
// Source: https://developers.openai.com/api/reference/resources/responses/methods/create
package responses

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const ContractSnapshot = "2026-08-24"

type Status string

const (
	StatusQueued     Status = "queued"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusIncomplete Status = "incomplete"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

type Truncation string

const (
	TruncationAuto     Truncation = "auto"
	TruncationDisabled Truncation = "disabled"
)

type CreateRequest struct {
	Model              string             `json:"model"`
	Input              Input              `json:"input"`
	Instructions       json.RawMessage    `json:"instructions,omitempty"`
	PreviousResponseID string             `json:"previous_response_id,omitempty"`
	Conversation       *ConversationParam `json:"conversation,omitempty"`
	Store              *bool              `json:"store,omitempty"`
	Stream             bool               `json:"stream,omitempty"`
	Background         bool               `json:"background,omitempty"`
	StreamOptions      *StreamOptions     `json:"stream_options,omitempty"`
	Tools              []Tool             `json:"tools,omitempty"`
	ToolChoice         json.RawMessage    `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool              `json:"parallel_tool_calls,omitempty"`
	MaxToolCalls       *int               `json:"max_tool_calls,omitempty"`
	MaxOutputTokens    *int               `json:"max_output_tokens,omitempty"`
	Reasoning          *ReasoningConfig   `json:"reasoning,omitempty"`
	Text               *TextConfig        `json:"text,omitempty"`
	Include            []Include          `json:"include,omitempty"`
	Metadata           map[string]string  `json:"metadata,omitempty"`
	Truncation         Truncation         `json:"truncation,omitempty"`
	Temperature        *float64           `json:"temperature,omitempty"`
	TopP               *float64           `json:"top_p,omitempty"`
	TopLogprobs        *int               `json:"top_logprobs,omitempty"`
	ServiceTier        string             `json:"service_tier,omitempty"`
	SafetyIdentifier   string             `json:"safety_identifier,omitempty"`
	PromptCacheKey     string             `json:"prompt_cache_key,omitempty"`
}

type ConversationParam struct {
	ID       string
	AsString bool
}

func (c *ConversationParam) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		c.AsString = true
		return json.Unmarshal(data, &c.ID)
	}
	var value struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("conversation must be an ID string or object: %w", err)
	}
	c.ID, c.AsString = value.ID, false
	return nil
}

func (c ConversationParam) MarshalJSON() ([]byte, error) {
	if c.AsString {
		return json.Marshal(c.ID)
	}
	return json.Marshal(struct {
		ID string `json:"id"`
	}{ID: c.ID})
}

type StreamOptions struct {
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

type ReasoningConfig struct {
	Effort          string `json:"effort,omitempty"`
	Summary         string `json:"summary,omitempty"`
	GenerateSummary string `json:"generate_summary,omitempty"`
}

type TextConfig struct {
	Format    *TextFormat `json:"format,omitempty"`
	Verbosity string      `json:"verbosity,omitempty"`
}

type TextFormat struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
}

type Include string

const (
	IncludeWebSearchSources       Include = "web_search_call.action.sources"
	IncludeCodeInterpreterOutputs Include = "code_interpreter_call.outputs"
	IncludeComputerImageURL       Include = "computer_call_output.output.image_url"
	IncludeFileSearchResults      Include = "file_search_call.results"
	IncludeInputImageURL          Include = "message.input_image.image_url"
	IncludeOutputTextLogprobs     Include = "message.output_text.logprobs"
	IncludeEncryptedReasoning     Include = "reasoning.encrypted_content"
)

// Input is the Responses API string-or-item-array union.
type Input struct {
	String *string
	Items  []InputItem
	Set    bool
}

func (i *Input) UnmarshalJSON(data []byte) error {
	i.Set = true
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("input must be a string or array")
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		i.String, i.Items = &value, nil
		return nil
	}
	var items []InputItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("input must be a string or array: %w", err)
	}
	i.String, i.Items = nil, items
	return nil
}

func (i Input) MarshalJSON() ([]byte, error) {
	if i.String != nil {
		return json.Marshal(*i.String)
	}
	if i.Items == nil {
		return []byte("null"), nil
	}
	return json.Marshal(i.Items)
}

type InputItem struct {
	Type      string          `json:"type,omitempty"`
	ID        string          `json:"id,omitempty"`
	Role      string          `json:"role,omitempty"`
	Status    Status          `json:"status,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Summary   []SummaryPart   `json:"summary,omitempty"`
	Encrypted string          `json:"encrypted_content,omitempty"`
}

type InputContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
	FileData string `json:"file_data,omitempty"`
	Filename string `json:"filename,omitempty"`
}

type OutputItem struct {
	Type             string          `json:"type"`
	ID               string          `json:"id"`
	Status           Status          `json:"status,omitempty"`
	Role             string          `json:"role,omitempty"`
	Content          []OutputContent `json:"content,omitempty"`
	CallID           string          `json:"call_id,omitempty"`
	Name             string          `json:"name,omitempty"`
	Arguments        string          `json:"arguments,omitempty"`
	Summary          []SummaryPart   `json:"summary,omitempty"`
	EncryptedContent string          `json:"encrypted_content,omitempty"`
	Action           json.RawMessage `json:"action,omitempty"`
	Results          json.RawMessage `json:"results,omitempty"`
	Outputs          json.RawMessage `json:"outputs,omitempty"`
}

type OutputContent struct {
	Type        string          `json:"type"`
	Text        string          `json:"text,omitempty"`
	Refusal     string          `json:"refusal,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
	Logprobs    json.RawMessage `json:"logprobs,omitempty"`
}

type SummaryPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Tool struct {
	Type              string          `json:"type"`
	Name              string          `json:"name,omitempty"`
	Description       string          `json:"description,omitempty"`
	Parameters        map[string]any  `json:"parameters,omitempty"`
	Strict            *bool           `json:"strict,omitempty"`
	VectorStoreIDs    []string        `json:"vector_store_ids,omitempty"`
	MaxNumResults     *int            `json:"max_num_results,omitempty"`
	Filters           json.RawMessage `json:"filters,omitempty"`
	SearchContextSize string          `json:"search_context_size,omitempty"`
}

type Response struct {
	ID                 string             `json:"id"`
	Object             string             `json:"object"`
	CreatedAt          int64              `json:"created_at"`
	CompletedAt        *int64             `json:"completed_at,omitempty"`
	Status             Status             `json:"status"`
	Error              *ResponseError     `json:"error"`
	IncompleteDetails  *IncompleteDetails `json:"incomplete_details"`
	Model              string             `json:"model"`
	PreviousResponseID *string            `json:"previous_response_id"`
	Instructions       json.RawMessage    `json:"instructions,omitempty"`
	Output             []OutputItem       `json:"output"`
	Tools              []Tool             `json:"tools"`
	ToolChoice         json.RawMessage    `json:"tool_choice"`
	ParallelToolCalls  bool               `json:"parallel_tool_calls"`
	Truncation         Truncation         `json:"truncation"`
	Reasoning          *ReasoningConfig   `json:"reasoning,omitempty"`
	Text               *TextConfig        `json:"text,omitempty"`
	Usage              *Usage             `json:"usage"`
	Metadata           map[string]string  `json:"metadata"`
	Store              bool               `json:"store"`
	Background         bool               `json:"background"`
	MaxOutputTokens    *int               `json:"max_output_tokens"`
	MaxToolCalls       *int               `json:"max_tool_calls,omitempty"`
}

type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type IncompleteDetails struct {
	Reason string `json:"reason"`
}

type Usage struct {
	InputTokens         int                `json:"input_tokens"`
	InputTokensDetails  InputTokenDetails  `json:"input_tokens_details"`
	OutputTokens        int                `json:"output_tokens"`
	OutputTokensDetails OutputTokenDetails `json:"output_tokens_details"`
	TotalTokens         int                `json:"total_tokens"`
}

type InputTokenDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type OutputTokenDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code,omitempty"`
}
