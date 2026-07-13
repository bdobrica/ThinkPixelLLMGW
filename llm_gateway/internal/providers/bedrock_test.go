package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type fakeBedrockClient struct {
	converseInput *bedrockruntime.ConverseInput
	converse      func(context.Context, *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error)
	countTokens   func(context.Context, *bedrockruntime.CountTokensInput) (*bedrockruntime.CountTokensOutput, error)
}

func (f *fakeBedrockClient) Converse(ctx context.Context, input *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	f.converseInput = input
	return f.converse(ctx, input)
}
func (f *fakeBedrockClient) ConverseStream(context.Context, *bedrockruntime.ConverseStreamInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error) {
	return nil, errors.New("not configured")
}
func (f *fakeBedrockClient) CountTokens(ctx context.Context, input *bedrockruntime.CountTokensInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.CountTokensOutput, error) {
	if f.countTokens == nil {
		return &bedrockruntime.CountTokensOutput{}, nil
	}
	return f.countTokens(ctx, input)
}

type staticAWSCredentials struct{ err error }

func (s staticAWSCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{AccessKeyID: "key", SecretAccessKey: "secret", Source: "test"}, s.err
}

func TestBedrockChatTranslatesRequestResponseAndUsage(t *testing.T) {
	client := &fakeBedrockClient{converse: func(_ context.Context, _ *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		return &bedrockruntime.ConverseOutput{
			Output:     &bedrocktypes.ConverseOutputMemberMessage{Value: bedrocktypes.Message{Role: bedrocktypes.ConversationRoleAssistant, Content: []bedrocktypes.ContentBlock{&bedrocktypes.ContentBlockMemberText{Value: "hello from bedrock"}}}},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage:      &bedrocktypes.TokenUsage{InputTokens: aws.Int32(13), OutputTokens: aws.Int32(5), TotalTokens: aws.Int32(18), CacheReadInputTokens: aws.Int32(4)},
		}, nil
	}}
	provider := &BedrockProvider{id: "bedrock", name: "Bedrock", region: "us-east-1", client: client, credentials: staticAWSCredentials{}}
	payload := map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "be concise"},
			map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "hello"}}},
		},
		"max_tokens": 100.0, "temperature": 0.3, "top_p": 0.8, "stop": []any{"END"},
	}
	response, err := provider.Chat(context.Background(), ChatRequest{Model: "anthropic.claude-3-haiku", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || response.InputTokens != 13 || response.OutputTokens != 5 || response.CachedTokens != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if aws.ToString(client.converseInput.ModelId) != "anthropic.claude-3-haiku" || len(client.converseInput.System) != 1 || len(client.converseInput.Messages) != 1 {
		t.Fatalf("unexpected translation: %+v", client.converseInput)
	}
	if aws.ToInt32(client.converseInput.InferenceConfig.MaxTokens) != 100 || aws.ToFloat32(client.converseInput.InferenceConfig.Temperature) != 0.3 || len(client.converseInput.InferenceConfig.StopSequences) != 1 {
		t.Fatalf("inference configuration not translated: %+v", client.converseInput.InferenceConfig)
	}
	var body struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(response.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Choices[0].Message.Content != "hello from bedrock" || body.Choices[0].FinishReason != "stop" || body.Usage.PromptTokens != 13 {
		t.Fatalf("unexpected OpenAI response: %+v", body)
	}
}

func TestBedrockRequestValidationAndErrorMapping(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload map[string]any
	}{
		{"missing messages", map[string]any{}},
		{"unsupported role", map[string]any{"messages": []any{map[string]any{"role": "tool", "content": "result"}}}},
		{"unsupported content", map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "image_url"}}}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := bedrockConverseInput(ChatRequest{Model: "model", Payload: test.payload}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	client := &fakeBedrockClient{converse: func(context.Context, *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		return nil, fakeHTTPError{status: 429}
	}}
	provider := &BedrockProvider{client: client}
	response, err := provider.Chat(context.Background(), ChatRequest{Model: "model", Payload: map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hi"}}}})
	if err != nil || response.StatusCode != 429 || !strings.Contains(string(response.Body), "bedrock_error") {
		t.Fatalf("unexpected mapped error: response=%+v err=%v", response, err)
	}
}

type fakeHTTPError struct{ status int }

func (f fakeHTTPError) Error() string       { return "cloud error" }
func (f fakeHTTPError) HTTPStatusCode() int { return f.status }

type fakeBedrockStreamReader struct {
	events chan bedrocktypes.ConverseStreamOutput
	err    error
	once   sync.Once
}

func (f *fakeBedrockStreamReader) Events() <-chan bedrocktypes.ConverseStreamOutput { return f.events }
func (f *fakeBedrockStreamReader) Err() error                                       { return f.err }
func (f *fakeBedrockStreamReader) Close() error                                     { f.once.Do(func() {}); return nil }

func TestBedrockStreamConvertsEventsAndTerminalUsage(t *testing.T) {
	events := make(chan bedrocktypes.ConverseStreamOutput, 4)
	events <- &bedrocktypes.ConverseStreamOutputMemberMessageStart{Value: bedrocktypes.MessageStartEvent{Role: bedrocktypes.ConversationRoleAssistant}}
	events <- &bedrocktypes.ConverseStreamOutputMemberContentBlockDelta{Value: bedrocktypes.ContentBlockDeltaEvent{ContentBlockIndex: aws.Int32(0), Delta: &bedrocktypes.ContentBlockDeltaMemberText{Value: "hello"}}}
	events <- &bedrocktypes.ConverseStreamOutputMemberMessageStop{Value: bedrocktypes.MessageStopEvent{StopReason: bedrocktypes.StopReasonMaxTokens}}
	events <- &bedrocktypes.ConverseStreamOutputMemberMetadata{Value: bedrocktypes.ConverseStreamMetadataEvent{Usage: &bedrocktypes.TokenUsage{InputTokens: aws.Int32(8), OutputTokens: aws.Int32(3), TotalTokens: aws.Int32(11), CacheReadInputTokens: aws.Int32(2)}, Metrics: &bedrocktypes.ConverseStreamMetrics{LatencyMs: aws.Int64(10)}}}
	close(events)
	stream := bedrockruntime.NewConverseStreamEventStream(func(options *bedrockruntime.ConverseStreamEventStream) {
		options.Reader = &fakeBedrockStreamReader{events: events}
	})
	reader := newBedrockSSEStream(context.Background(), func() {}, stream, "model")
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{`"role":"assistant"`, `"content":"hello"`, `"finish_reason":"length"`, `"prompt_tokens":8`, `"cached_tokens":2`, "data: [DONE]"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("stream missing %q:\n%s", expected, text)
		}
	}
}

func TestBedrockCredentialValidation(t *testing.T) {
	t.Run("credential chain", func(t *testing.T) {
		provider := &BedrockProvider{credentials: staticAWSCredentials{}}
		if err := provider.ValidateCredentials(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid credentials", func(t *testing.T) {
		provider := &BedrockProvider{credentials: staticAWSCredentials{err: errors.New("expired")}}
		if err := provider.ValidateCredentials(context.Background()); err == nil {
			t.Fatal("expected credential error")
		}
	})
	t.Run("permission check", func(t *testing.T) {
		called := false
		client := &fakeBedrockClient{countTokens: func(_ context.Context, input *bedrockruntime.CountTokensInput) (*bedrockruntime.CountTokensOutput, error) {
			called = aws.ToString(input.ModelId) == "validation-model"
			return &bedrockruntime.CountTokensOutput{}, nil
		}}
		provider := &BedrockProvider{credentials: staticAWSCredentials{}, client: client, validationModel: "validation-model"}
		if err := provider.ValidateCredentials(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("CountTokens permission validation was not called")
		}
	})
}

func TestBedrockChatHonorsCancellation(t *testing.T) {
	client := &fakeBedrockClient{converse: func(ctx context.Context, _ *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	provider := &BedrockProvider{client: client}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := provider.Chat(ctx, ChatRequest{Model: "model", Payload: map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hello"}}}})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestBedrockChatHonorsProviderTimeout(t *testing.T) {
	client := &fakeBedrockClient{converse: func(ctx context.Context, _ *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	provider := &BedrockProvider{client: client, requestTimeout: 20 * time.Millisecond}
	_, err := provider.Chat(context.Background(), ChatRequest{Model: "model", Payload: map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hello"}}}})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected provider timeout error, got %v", err)
	}
}
