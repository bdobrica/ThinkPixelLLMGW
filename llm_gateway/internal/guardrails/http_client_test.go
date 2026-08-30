package guardrails

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testHTTPClient(t *testing.T, response func(*http.Request) *http.Response, maxBytes int64) *HTTPClient {
	t.Helper()
	client, err := NewHTTPClient(HTTPClientConfig{
		Endpoint:         "https://guardrails.example/prefix/",
		BearerToken:      "secret",
		MaxResponseBytes: maxBytes,
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return response(r), nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestHTTPClientEvaluate(t *testing.T) {
	client := testHTTPClient(t, func(r *http.Request) *http.Response {
		if r.URL.Path != "/prefix/v1/evaluations" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		return jsonResponse(http.StatusOK, `{"evaluation_id":"eval-1","request_id":"req-1","decision":{"action":"allow","reason":"ok"},"applied_policies":[],"findings":[],"timing":{}}`)
	}, 0)

	result, err := client.Evaluate(context.Background(), EvaluationRequest{RequestID: "req-1", Stage: StagePreModel, Content: Content{Text: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ActionAllow {
		t.Fatalf("unexpected action %q", result.Decision.Action)
	}
}

func TestHTTPClientRejectsUnsafeResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "mismatched request", body: `{"evaluation_id":"eval-1","request_id":"other","decision":{"action":"allow","reason":"ok"}}`, want: "request_id mismatch"},
		{name: "unknown action", body: `{"evaluation_id":"eval-1","request_id":"req-1","decision":{"action":"review","reason":"maybe"}}`, want: "unsupported guardrails action"},
		{name: "redact without content", body: `{"evaluation_id":"eval-1","request_id":"req-1","decision":{"action":"redact","reason":"pii"}}`, want: "requires transformed_content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testHTTPClient(t, func(*http.Request) *http.Response { return jsonResponse(http.StatusOK, tt.body) }, 0)
			_, err := client.Evaluate(context.Background(), EvaluationRequest{RequestID: "req-1", Stage: StagePostModel})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestHTTPClientBoundsAndSanitizesErrors(t *testing.T) {
	client := testHTTPClient(t, func(*http.Request) *http.Response {
		return jsonResponse(http.StatusBadRequest, "sensitive policy details")
	}, 8)
	_, err := client.Evaluate(context.Background(), EvaluationRequest{RequestID: "req-1", Stage: StagePreModel})
	if err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("expected sanitized bounded error, got %v", err)
	}
}
