package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/auth"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/logging"
	"github.com/bdobrica/ThinkPixelLLMGW/llm-gateway/internal/providers"
)

// handleChat is the entry point for OpenAI-compatible chat completions.
//
// Flow:
//  1. Validate method
//  2. Authenticate via Bearer API key
//  3. Decode JSON body
//  4. Extract model + check key permissions
//  5. Rate limit
//  6. Budget check
//  7. Resolve model → provider + providerModel
//  8. Call provider
//  9. Log + update billing
// 10. Return provider response
func (d *Dependencies) handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := newRequestID()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// 1. Auth via "Authorization: Bearer <key>"
	plaintextKey, err := parseBearer(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	apiKey, err := d.APIKeys.Lookup(ctx, plaintextKey)
	if err != nil {
		if errors.Is(err, auth.ErrKeyNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	if apiKey.Revoked {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Decode request body as generic JSON (OpenAI-style payload).
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	// 3. Extract model name.
	modelName, _ := payload["model"].(string)
	if modelName == "" {
		http.Error(w, "missing model field", http.StatusBadRequest)
		return
	}

	// 4. Check if key is allowed to call this model/alias.
	if !apiKey.AllowsModel(modelName) {
		http.Error(w, "model not allowed for this key", http.StatusForbidden)
		return
	}

	// 5. Rate limit.
	if ok := d.RateLimit.Allow(ctx, apiKey.ID); !ok {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// 6. Budget check.
	if ok := d.Billing.WithinBudget(ctx, apiKey.ID); !ok {
		http.Error(w, "budget exceeded", http.StatusPaymentRequired)
		return
	}

	// 7. Resolve model → provider + providerModel.
	provider, providerModel, err := d.Providers.ResolveModel(ctx, modelName)
	if err != nil || provider == nil {
		http.Error(w, "unknown model", http.StatusBadRequest)
		return
	}

	// 8. Call provider.
	pReq := providers.ChatRequest{
		Model:   providerModel,
		Payload: payload,
	}

	pStart := time.Now()
	pResp, err := provider.Chat(ctx, pReq)
	providerLatency := time.Since(pStart)

	// 9. Prepare a log record either way.
	logRec := &logging.Record{
		Timestamp:      time.Now(),
		RequestID:      reqID,
		APIKeyID:       apiKey.ID,
		APIKeyName:     apiKey.Name,
		Provider:       string(provider.Type()),
		Model:          providerModel,
		Alias:          modelName,
		Tags:           apiKey.Tags,
		ProviderMs:     providerLatency.Milliseconds(),
		GatewayMs:      time.Since(start).Milliseconds(),
		RequestPayload: payload,
	}

	if err != nil {
		logRec.Error = err.Error()
		_ = d.Logger.Enqueue(logRec) // best-effort

		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	logRec.CostUSD = pResp.CostUSD
	// TODO: optionally decode response JSON and store a summarized form instead of raw bytes
	logRec.ResponsePayload = json.RawMessage(pResp.Body)

	// Update billing (best-effort, errors can be logged and ignored for the response)
	_ = d.Billing.AddUsage(ctx, apiKey.ID, pResp.CostUSD)

	_ = d.Logger.Enqueue(logRec)

	// TODO: integrate real metrics here (e.g. Prometheus counters/histograms).

	// 10. Return provider response as-is (transparent proxy).
	for k, vals := range r.Header {
		// You may later copy some headers from provider instead.
		_ = vals
		_ = k
		// TODO: copy relevant headers from pResp if needed.
	}

	w.WriteHeader(pResp.StatusCode)
	_, _ = w.Write(pResp.Body)
}

// parseBearer extracts the token from an Authorization: Bearer <token> header.
func parseBearer(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization header")
	}
	if parts[1] == "" {
		return "", errors.New("empty bearer token")
	}
	return parts[1], nil
}

// newRequestID returns a simple request ID.
// You can swap this later for a UUID library or trace ID.
func newRequestID() string {
	return time.Now().Format(time.RFC3339Nano)
}
