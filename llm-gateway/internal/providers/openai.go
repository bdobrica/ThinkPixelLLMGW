package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openAIDefaultBaseURL = "https://api.openai.com/v1"
	openAITimeout        = 60 * time.Second
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	id      string
	name    string
	auth    Authenticator
	client  *http.Client
	baseURL string
}

// NewOpenAIProvider creates a new OpenAI provider instance
func NewOpenAIProvider(config ProviderConfig) (Provider, error) {
	// Extract API key from credentials
	apiKey, ok := config.Credentials["api_key"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("api_key is required for OpenAI provider")
	}

	// Get base URL from config or use default
	baseURL := openAIDefaultBaseURL
	if url, ok := config.Config["base_url"].(string); ok && url != "" {
		baseURL = url
	}

	// Create authenticator
	auth := NewSimpleAPIKeyAuth(apiKey, "Authorization", "Bearer ")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: openAITimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &OpenAIProvider{
		id:      config.ID,
		name:    config.Name,
		auth:    auth,
		client:  client,
		baseURL: baseURL,
	}, nil
}

// ID returns the provider ID
func (p *OpenAIProvider) ID() string {
	return p.id
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return p.name
}

// Type returns the provider type
func (p *OpenAIProvider) Type() string {
	return "openai"
}

// Chat sends a chat completion request to OpenAI
func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Build OpenAI request
	openAIReq := req.Payload
	if openAIReq["model"] == nil {
		openAIReq["model"] = req.Model
	}

	// Handle streaming
	isStream := req.Stream
	if stream, ok := openAIReq["stream"].(bool); ok {
		isStream = stream
	}

	// Marshal request body
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Apply authentication
	authCtx, err := p.auth.Authenticate(ctx)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	if err := authCtx.ApplyToRequest(ctx, httpReq); err != nil {
		return nil, fmt.Errorf("failed to apply auth: %w", err)
	}

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	latency := time.Since(start)

	// Handle non-streaming response
	if !isStream {
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Calculate cost from usage in response
		cost := 0.0
		if resp.StatusCode == http.StatusOK {
			cost = extractCostFromResponse(respBody)
		}

		return &ChatResponse{
			StatusCode:      resp.StatusCode,
			Body:            respBody,
			ProviderLatency: latency,
			CostUSD:         cost,
		}, nil
	}

	// Handle streaming response
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return &ChatResponse{
			StatusCode:      resp.StatusCode,
			Body:            respBody,
			ProviderLatency: latency,
		}, nil
	}

	// Return streaming response
	return &ChatResponse{
		StatusCode:      resp.StatusCode,
		Stream:          resp.Body,
		ProviderLatency: latency,
	}, nil
}

// ValidateCredentials validates the provider credentials
func (p *OpenAIProvider) ValidateCredentials(ctx context.Context) error {
	// Make a simple API call to validate credentials
	url := p.baseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	authCtx, err := p.auth.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := authCtx.ApplyToRequest(ctx, httpReq); err != nil {
		return fmt.Errorf("failed to apply auth: %w", err)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validation failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// Close cleans up resources
func (p *OpenAIProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}

// extractCostFromResponse extracts token usage and calculates cost
// In production, you'd use the model pricing from database
func extractCostFromResponse(body []byte) float64 {
	var response struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return 0.0
	}

	// This is a placeholder calculation
	// In production, fetch model pricing from database and use Model.CalculateCost()
	inputCost := float64(response.Usage.PromptTokens) * 0.00001      // $0.01 per 1K tokens
	outputCost := float64(response.Usage.CompletionTokens) * 0.00003 // $0.03 per 1K tokens

	return inputCost + outputCost
}

// StreamReader provides a convenient way to read streaming responses
type StreamReader struct {
	scanner *bufio.Scanner
	closer  io.Closer
}

// NewStreamReader creates a new stream reader
func NewStreamReader(r io.ReadCloser) *StreamReader {
	return &StreamReader{
		scanner: bufio.NewScanner(r),
		closer:  r,
	}
}

// Read reads the next event from the stream
func (s *StreamReader) Read() (*StreamEvent, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return &StreamEvent{Error: err}, err
		}
		return &StreamEvent{Done: true}, io.EOF
	}

	line := s.scanner.Bytes()

	// Skip empty lines
	if len(line) == 0 {
		return s.Read()
	}

	// Check for data: prefix
	if !bytes.HasPrefix(line, []byte("data: ")) {
		return s.Read()
	}

	data := bytes.TrimPrefix(line, []byte("data: "))

	// Check for [DONE] marker
	if bytes.Equal(data, []byte("[DONE]")) {
		return &StreamEvent{Done: true}, io.EOF
	}

	return &StreamEvent{Data: data}, nil
}

// Close closes the stream
func (s *StreamReader) Close() error {
	return s.closer.Close()
}
