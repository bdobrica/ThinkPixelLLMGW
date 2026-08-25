package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const vertexCloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// VertexAIProvider calls Vertex AI's OpenAI-compatible Chat Completions API.
// The compatibility endpoint lets the gateway preserve its public request and
// streaming contracts while Google OAuth handles short-lived access tokens.
type VertexAIProvider struct {
	id            string
	name          string
	projectID     string
	location      string
	client        *http.Client
	baseURL       string
	validationURL string
}

// NewVertexAIProvider creates a Vertex AI provider using a service-account JSON
// key, a supplied short-lived access token, or Application Default Credentials.
func NewVertexAIProvider(config ProviderConfig) (Provider, error) {
	projectID, _ := config.Config["project_id"].(string)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required for Vertex AI provider")
	}

	location, _ := config.Config["location"].(string)
	if location == "" {
		location = "us-central1"
	}

	tokenSource, err := vertexTokenSource(config.Credentials)
	if err != nil {
		return nil, err
	}

	apiHost := vertexAPIHost(location)
	baseURL := fmt.Sprintf("https://%s/v1/projects/%s/locations/%s/endpoints/openapi", apiHost, projectID, location)
	if configured, _ := config.Config["base_url"].(string); configured != "" {
		baseURL = strings.TrimRight(configured, "/")
	}
	validationURL := fmt.Sprintf("https://%s/v1/projects/%s/locations/%s/publishers/google/models?pageSize=1", apiHost, projectID, location)
	if configured, _ := config.Config["validation_url"].(string); configured != "" {
		validationURL = configured
	}

	client := oauth2.NewClient(context.Background(), oauth2.ReuseTokenSource(nil, tokenSource))
	client.Timeout = parseProviderTimeout(config.Config)

	return &VertexAIProvider{
		id:            config.ID,
		name:          config.Name,
		projectID:     projectID,
		location:      location,
		client:        client,
		baseURL:       baseURL,
		validationURL: validationURL,
	}, nil
}

func vertexAPIHost(location string) string {
	if location == "global" {
		return "aiplatform.googleapis.com"
	}
	return location + "-aiplatform.googleapis.com"
}

func vertexTokenSource(credentials map[string]string) (oauth2.TokenSource, error) {
	if accessToken := credentials["access_token"]; accessToken != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken, TokenType: "Bearer"}), nil
	}

	serviceAccountJSON := credentials["service_account_json"]
	if serviceAccountJSON == "" {
		serviceAccountJSON = credentials["credentials_json"]
	}
	if serviceAccountJSON != "" {
		creds, err := google.CredentialsFromJSON(context.Background(), []byte(serviceAccountJSON), vertexCloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("invalid Vertex AI service account credentials: %w", err)
		}
		return creds.TokenSource, nil
	}

	creds, err := google.FindDefaultCredentials(context.Background(), vertexCloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI credentials are required (service_account_json, access_token, or application default credentials): %w", err)
	}
	return creds.TokenSource, nil
}

func (p *VertexAIProvider) ID() string   { return p.id }
func (p *VertexAIProvider) Name() string { return p.name }
func (p *VertexAIProvider) Type() string { return "vertexai" }

// Chat forwards the normalized OpenAI payload to Vertex AI. The provider
// response is already OpenAI-compatible, including terminal stream usage.
func (p *VertexAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	start := time.Now()
	payload := clonePayload(req.Payload)
	if payload["model"] == nil {
		payload["model"] = req.Model
	}
	isStream := req.Stream
	if stream, ok := payload["stream"].(bool); ok {
		isStream = stream
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Vertex AI request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI request failed: %w", err)
	}
	latency := time.Since(start)
	if isStream && resp.StatusCode == http.StatusOK {
		return &ChatResponse{StatusCode: resp.StatusCode, Stream: resp.Body, ProviderLatency: latency}, nil
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read Vertex AI response: %w", readErr)
	}
	usage := &UsageInfo{}
	if resp.StatusCode == http.StatusOK {
		usage = extractUsageFromResponse(responseBody)
	}
	return &ChatResponse{
		StatusCode: resp.StatusCode, Body: responseBody, ProviderLatency: latency,
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		CachedTokens: usage.CachedTokens, ReasoningTokens: usage.ReasoningTokens,
	}, nil
}

// ValidateCredentials verifies both OAuth token acquisition and Vertex AI API
// access without running a billable generation request.
func (p *VertexAIProvider) ValidateCredentials(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.validationURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Vertex AI validation request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("Vertex AI credential validation failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return fmt.Errorf("Vertex AI credential validation failed: status=%d, body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (p *VertexAIProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}

func clonePayload(payload map[string]any) map[string]any {
	cloned := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
