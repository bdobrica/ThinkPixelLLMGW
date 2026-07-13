package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type bedrockRuntimeAPI interface {
	Converse(context.Context, *bedrockruntime.ConverseInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
	ConverseStream(context.Context, *bedrockruntime.ConverseStreamInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
	CountTokens(context.Context, *bedrockruntime.CountTokensInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.CountTokensOutput, error)
}

// BedrockProvider uses the model-independent Bedrock Converse API. AWS SDK v2
// supplies SigV4 signing, credential refresh, IAM roles, web identity, SSO, and
// the standard environment/shared-config credential chain.
type BedrockProvider struct {
	id              string
	name            string
	region          string
	client          bedrockRuntimeAPI
	credentials     aws.CredentialsProvider
	validationModel string
	requestTimeout  time.Duration
}

func NewBedrockProvider(config ProviderConfig) (Provider, error) {
	region, _ := config.Config["region"].(string)
	if region == "" {
		region = "us-east-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	accessKey := config.Credentials["access_key_id"]
	secretKey := config.Credentials["secret_access_key"]
	if (accessKey == "") != (secretKey == "") {
		return nil, fmt.Errorf("both access_key_id and secret_access_key are required when using static AWS credentials")
	}
	if accessKey != "" {
		provider := credentials.NewStaticCredentialsProvider(accessKey, secretKey, config.Credentials["session_token"])
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(provider))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS credentials for Bedrock: %w", err)
	}
	client := bedrockruntime.NewFromConfig(awsCfg, func(options *bedrockruntime.Options) {
		if endpoint, _ := config.Config["base_url"].(string); endpoint != "" {
			options.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
		}
	})
	validationModel, _ := config.Config["validation_model"].(string)

	return &BedrockProvider{
		id: config.ID, name: config.Name, region: region, client: client,
		credentials: awsCfg.Credentials, validationModel: validationModel,
		requestTimeout: parseProviderTimeout(config.Config),
	}, nil
}

func (p *BedrockProvider) ID() string   { return p.id }
func (p *BedrockProvider) Name() string { return p.name }
func (p *BedrockProvider) Type() string { return "bedrock" }

func (p *BedrockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	timeout := p.requestTimeout
	if timeout <= 0 {
		timeout = openAITimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	input, err := bedrockConverseInput(req)
	if err != nil {
		cancel()
		return nil, err
	}
	isStream := req.Stream
	if value, ok := req.Payload["stream"].(bool); ok {
		isStream = value
	}

	if isStream {
		streamInput := &bedrockruntime.ConverseStreamInput{
			ModelId: input.ModelId, Messages: input.Messages, System: input.System,
			InferenceConfig: input.InferenceConfig,
		}
		output, callErr := p.client.ConverseStream(requestCtx, streamInput)
		if callErr != nil {
			cancel()
			if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) {
				return nil, fmt.Errorf("Bedrock request failed: %w", callErr)
			}
			return bedrockErrorResponse(callErr, time.Since(start)), nil
		}
		return &ChatResponse{
			StatusCode:      http.StatusOK,
			Stream:          newBedrockSSEStream(requestCtx, cancel, output.GetStream(), req.Model),
			ProviderLatency: time.Since(start),
		}, nil
	}

	defer cancel()
	output, callErr := p.client.Converse(requestCtx, input)
	if callErr != nil {
		if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("Bedrock request failed: %w", callErr)
		}
		return bedrockErrorResponse(callErr, time.Since(start)), nil
	}
	responseBody, usage, err := bedrockOpenAIResponse(output, req.Model)
	if err != nil {
		return nil, err
	}
	return &ChatResponse{
		StatusCode: http.StatusOK, Body: responseBody, ProviderLatency: time.Since(start),
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, CachedTokens: usage.CachedTokens,
	}, nil
}

func bedrockConverseInput(req ChatRequest) (*bedrockruntime.ConverseInput, error) {
	model := req.Model
	if payloadModel, ok := req.Payload["model"].(string); ok && payloadModel != "" {
		model = payloadModel
	}
	if model == "" {
		return nil, fmt.Errorf("model is required for Bedrock provider")
	}

	messages, system, err := bedrockMessages(req.Payload["messages"])
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one user or assistant message is required for Bedrock")
	}
	inference := &bedrocktypes.InferenceConfiguration{}
	if value, ok := numericValue(req.Payload["max_completion_tokens"]); ok {
		inference.MaxTokens = aws.Int32(int32(value))
	} else if value, ok := numericValue(req.Payload["max_tokens"]); ok {
		inference.MaxTokens = aws.Int32(int32(value))
	}
	if value, ok := floatValue(req.Payload["temperature"]); ok {
		inference.Temperature = aws.Float32(float32(value))
	}
	if value, ok := floatValue(req.Payload["top_p"]); ok {
		inference.TopP = aws.Float32(float32(value))
	}
	switch stop := req.Payload["stop"].(type) {
	case string:
		inference.StopSequences = []string{stop}
	case []string:
		inference.StopSequences = stop
	case []any:
		for _, item := range stop {
			if value, ok := item.(string); ok {
				inference.StopSequences = append(inference.StopSequences, value)
			}
		}
	}
	return &bedrockruntime.ConverseInput{ModelId: aws.String(model), Messages: messages, System: system, InferenceConfig: inference}, nil
}

func bedrockMessages(raw any) ([]bedrocktypes.Message, []bedrocktypes.SystemContentBlock, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, nil, fmt.Errorf("messages must be an array for Bedrock provider")
	}
	var messages []bedrocktypes.Message
	var system []bedrocktypes.SystemContentBlock
	for index, item := range items {
		message, ok := item.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("message %d must be an object", index)
		}
		role, _ := message["role"].(string)
		text, err := openAIMessageText(message["content"])
		if err != nil {
			return nil, nil, fmt.Errorf("message %d: %w", index, err)
		}
		if role == "system" || role == "developer" {
			system = append(system, &bedrocktypes.SystemContentBlockMemberText{Value: text})
			continue
		}
		var bedrockRole bedrocktypes.ConversationRole
		switch role {
		case "user":
			bedrockRole = bedrocktypes.ConversationRoleUser
		case "assistant":
			bedrockRole = bedrocktypes.ConversationRoleAssistant
		default:
			return nil, nil, fmt.Errorf("message %d has unsupported Bedrock role %q", index, role)
		}
		messages = append(messages, bedrocktypes.Message{
			Role:    bedrockRole,
			Content: []bedrocktypes.ContentBlock{&bedrocktypes.ContentBlockMemberText{Value: text}},
		})
	}
	return messages, system, nil
}

func openAIMessageText(content any) (string, error) {
	if text, ok := content.(string); ok {
		return text, nil
	}
	parts, ok := content.([]any)
	if !ok {
		return "", fmt.Errorf("content must be text or an array of text parts")
	}
	var text strings.Builder
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok || part["type"] != "text" {
			return "", fmt.Errorf("only text content parts are currently supported by the Bedrock adapter")
		}
		value, _ := part["text"].(string)
		text.WriteString(value)
	}
	return text.String(), nil
}

func bedrockOpenAIResponse(output *bedrockruntime.ConverseOutput, model string) ([]byte, UsageInfo, error) {
	messageOutput, ok := output.Output.(*bedrocktypes.ConverseOutputMemberMessage)
	if !ok {
		return nil, UsageInfo{}, fmt.Errorf("Bedrock returned an unsupported response type %T", output.Output)
	}
	var content strings.Builder
	for _, block := range messageOutput.Value.Content {
		if text, ok := block.(*bedrocktypes.ContentBlockMemberText); ok {
			content.WriteString(text.Value)
		}
	}
	usage := bedrockUsage(output.Usage)
	body := map[string]any{
		"id": fmt.Sprintf("bedrock-%d", time.Now().UnixNano()), "object": "chat.completion",
		"created": time.Now().Unix(), "model": model,
		"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": content.String()}, "finish_reason": bedrockFinishReason(output.StopReason)}},
		"usage":   openAIUsage(usage),
	}
	encoded, err := json.Marshal(body)
	return encoded, usage, err
}

func bedrockUsage(usage *bedrocktypes.TokenUsage) UsageInfo {
	if usage == nil {
		return UsageInfo{}
	}
	result := UsageInfo{}
	if usage.InputTokens != nil {
		result.InputTokens = int(*usage.InputTokens)
	}
	if usage.OutputTokens != nil {
		result.OutputTokens = int(*usage.OutputTokens)
	}
	if usage.TotalTokens != nil {
		result.TotalTokens = int(*usage.TotalTokens)
	}
	if usage.CacheReadInputTokens != nil {
		result.CachedTokens = int(*usage.CacheReadInputTokens)
	}
	return result
}

func openAIUsage(usage UsageInfo) map[string]any {
	return map[string]any{
		"prompt_tokens": usage.InputTokens, "completion_tokens": usage.OutputTokens,
		"total_tokens":          usage.TotalTokens,
		"prompt_tokens_details": map[string]any{"cached_tokens": usage.CachedTokens},
	}
}

func bedrockFinishReason(reason bedrocktypes.StopReason) string {
	switch reason {
	case bedrocktypes.StopReasonMaxTokens:
		return "length"
	case bedrocktypes.StopReasonToolUse:
		return "tool_calls"
	case bedrocktypes.StopReasonGuardrailIntervened, bedrocktypes.StopReasonContentFiltered:
		return "content_filter"
	default:
		return "stop"
	}
}

type bedrockSSEStream struct {
	reader *io.PipeReader
	stream *bedrockruntime.ConverseStreamEventStream
	cancel context.CancelFunc
	once   sync.Once
}

func newBedrockSSEStream(ctx context.Context, cancel context.CancelFunc, stream *bedrockruntime.ConverseStreamEventStream, model string) io.ReadCloser {
	reader, writer := io.Pipe()
	result := &bedrockSSEStream{reader: reader, stream: stream, cancel: cancel}
	go result.writeEvents(ctx, writer, model)
	return result
}

func (s *bedrockSSEStream) writeEvents(ctx context.Context, writer *io.PipeWriter, model string) {
	defer writer.Close()
	defer s.stream.Close()
	defer s.cancel()
	writeChunk := func(chunk map[string]any) bool {
		encoded, err := json.Marshal(chunk)
		if err != nil {
			return false
		}
		_, err = fmt.Fprintf(writer, "data: %s\n\n", encoded)
		return err == nil
	}
	base := func() map[string]any {
		return map[string]any{"id": fmt.Sprintf("bedrock-%d", time.Now().UnixNano()), "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model}
	}
	for {
		select {
		case <-ctx.Done():
			writer.CloseWithError(ctx.Err())
			return
		case event, ok := <-s.stream.Events():
			if !ok {
				if err := s.stream.Err(); err != nil {
					writer.CloseWithError(err)
					return
				}
				fmt.Fprint(writer, "data: [DONE]\n\n")
				return
			}
			chunk := base()
			switch value := event.(type) {
			case *bedrocktypes.ConverseStreamOutputMemberMessageStart:
				chunk["choices"] = []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}}
			case *bedrocktypes.ConverseStreamOutputMemberContentBlockDelta:
				textDelta, ok := value.Value.Delta.(*bedrocktypes.ContentBlockDeltaMemberText)
				if !ok {
					continue
				}
				chunk["choices"] = []any{map[string]any{"index": 0, "delta": map[string]any{"content": textDelta.Value}, "finish_reason": nil}}
			case *bedrocktypes.ConverseStreamOutputMemberMessageStop:
				chunk["choices"] = []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": bedrockFinishReason(value.Value.StopReason)}}
			case *bedrocktypes.ConverseStreamOutputMemberMetadata:
				chunk["choices"] = []any{}
				chunk["usage"] = openAIUsage(bedrockUsage(value.Value.Usage))
			default:
				continue
			}
			if !writeChunk(chunk) {
				return
			}
		}
	}
}

func (s *bedrockSSEStream) Read(buffer []byte) (int, error) { return s.reader.Read(buffer) }
func (s *bedrockSSEStream) Close() error {
	var err error
	s.once.Do(func() {
		s.cancel()
		err = errors.Join(s.reader.Close(), s.stream.Close())
	})
	return err
}

func bedrockErrorResponse(err error, latency time.Duration) *ChatResponse {
	status := http.StatusBadGateway
	var httpError interface{ HTTPStatusCode() int }
	if errors.As(err, &httpError) && httpError.HTTPStatusCode() > 0 {
		status = httpError.HTTPStatusCode()
	}
	body, _ := json.Marshal(map[string]any{"error": map[string]any{"message": err.Error(), "type": "bedrock_error"}})
	return &ChatResponse{StatusCode: status, Body: body, ProviderLatency: latency}
}

func (p *BedrockProvider) ValidateCredentials(ctx context.Context) error {
	if _, err := p.credentials.Retrieve(ctx); err != nil {
		return fmt.Errorf("AWS credential validation failed: %w", err)
	}
	if p.validationModel == "" {
		return nil
	}
	_, err := p.client.CountTokens(ctx, &bedrockruntime.CountTokensInput{
		ModelId: aws.String(p.validationModel),
		Input: &bedrocktypes.CountTokensInputMemberConverse{Value: bedrocktypes.ConverseTokensRequest{
			Messages: []bedrocktypes.Message{{Role: bedrocktypes.ConversationRoleUser, Content: []bedrocktypes.ContentBlock{&bedrocktypes.ContentBlockMemberText{Value: "credential validation"}}}},
		}},
	})
	if err != nil {
		return fmt.Errorf("Bedrock permission validation failed: %w", err)
	}
	return nil
}

func (p *BedrockProvider) Close() error { return nil }

func numericValue(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, number > 0
	case int64:
		return int(number), number > 0
	case float64:
		return int(number), number > 0
	default:
		return 0, false
	}
}

func floatValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	default:
		return 0, false
	}
}
