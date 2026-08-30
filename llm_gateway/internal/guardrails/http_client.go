package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultMaxResponseBytes int64 = 1 << 20
const defaultRequestTimeout = 2 * time.Second

type HTTPClient struct {
	endpoint         *url.URL
	token            string
	client           *http.Client
	maxResponseBytes int64
}

type HTTPClientConfig struct {
	Endpoint         string
	BearerToken      string
	Client           *http.Client
	Timeout          time.Duration
	MaxResponseBytes int64
}

func NewHTTPClient(cfg HTTPClientConfig) (*HTTPClient, error) {
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, errors.New("guardrails endpoint must be an absolute HTTP URL")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, errors.New("guardrails endpoint must use http or https")
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/v1/evaluations"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	client := cfg.Client
	if client == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = defaultRequestTimeout
		}
		if timeout < 0 {
			return nil, errors.New("guardrails timeout must be positive")
		}
		client = &http.Client{Timeout: timeout}
	} else if cfg.Timeout != 0 {
		return nil, errors.New("guardrails timeout cannot be set with a custom HTTP client")
	}
	maxBytes := cfg.MaxResponseBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxResponseBytes
	}
	if maxBytes < 0 {
		return nil, errors.New("guardrails maximum response size must be positive")
	}
	return &HTTPClient{endpoint: endpoint, token: cfg.BearerToken, client: client, maxResponseBytes: maxBytes}, nil
}

func (c *HTTPClient) Evaluate(ctx context.Context, evaluation EvaluationRequest) (*EvaluationResponse, error) {
	if evaluation.RequestID == "" {
		return nil, errors.New("guardrails request_id is required")
	}
	if evaluation.Stage != StagePreModel && evaluation.Stage != StagePostModel {
		return nil, fmt.Errorf("unsupported guardrails stage %q", evaluation.Stage)
	}
	body, err := json.Marshal(evaluation)
	if err != nil {
		return nil, fmt.Errorf("encode guardrails request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create guardrails request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("guardrails request failed: %w", err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, c.maxResponseBytes+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read guardrails response: %w", err)
	}
	if int64(len(responseBody)) > c.maxResponseBytes {
		return nil, errors.New("guardrails response exceeds configured limit")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("guardrails returned HTTP %d", resp.StatusCode)
	}
	var result EvaluationResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("decode guardrails response: %w", err)
	}
	if result.RequestID != evaluation.RequestID {
		return nil, errors.New("guardrails response request_id mismatch")
	}
	if result.EvaluationID == "" {
		return nil, errors.New("guardrails response evaluation_id is required")
	}
	switch result.Decision.Action {
	case ActionAllow, ActionBlock, ActionMonitor:
	case ActionRedact:
		if result.Decision.TransformedContent == nil {
			return nil, errors.New("guardrails redact decision requires transformed_content")
		}
	default:
		return nil, fmt.Errorf("unsupported guardrails action %q", result.Decision.Action)
	}
	return &result, nil
}
