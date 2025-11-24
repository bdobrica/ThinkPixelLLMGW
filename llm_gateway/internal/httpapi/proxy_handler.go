package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/logging"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
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
//
// 10. Return provider response
func (d *Dependencies) handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := newRequestID()

	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	// 1. Auth via "Authorization: Bearer <key>"
	plaintextKey, err := parseBearer(r.Header.Get("Authorization"))
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
		return
	}

	// 2. Lookup API key from database (with caching)
	apiKeyRecord, err := d.APIKeys.Lookup(ctx, plaintextKey)
	if err != nil {
		if errors.Is(err, storage.ErrAPIKeyNotFound) {
			writeJSONError(w, http.StatusUnauthorized, "invalid API key")
		} else {
			writeJSONError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	// Check if key is enabled and not expired
	if !apiKeyRecord.Enabled {
		writeJSONError(w, http.StatusUnauthorized, "API key disabled")
		return
	}
	if apiKeyRecord.ExpiresAt != nil && apiKeyRecord.ExpiresAt.Before(time.Now()) {
		writeJSONError(w, http.StatusUnauthorized, "API key expired")
		return
	}

	// 3. Decode request body as generic JSON (OpenAI-style payload).
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// 4. Extract model name.
	modelName, _ := payload["model"].(string)
	if modelName == "" {
		writeJSONError(w, http.StatusBadRequest, "missing 'model' field")
		return
	}

	// Check if streaming is requested
	isStreaming, _ := payload["stream"].(bool)

	// 5. Check if key is allowed to call this model/alias.
	// TODO: Implement AllowedModels check when APIKey has that field
	// For now, we allow all models

	// 6. Rate limit check
	allowed := d.RateLimit.Allow(ctx, apiKeyRecord.ID.String())
	if !allowed {
		writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	// 7. Budget check
	withinBudget := d.Billing.WithinBudget(ctx, apiKeyRecord.ID.String())
	if !withinBudget {
		writeJSONError(w, http.StatusPaymentRequired, "monthly budget exceeded")
		return
	}

	// 8. Resolve model → provider + providerModel
	provider, providerModel, err := d.Providers.ResolveModel(ctx, modelName)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("unknown model: %s", modelName))
		return
	}

	// 9. Call provider
	pReq := providers.ChatRequest{
		Model:   providerModel,
		Payload: payload,
		Stream:  isStreaming,
	}

	pStart := time.Now()
	pResp, err := provider.Chat(ctx, pReq)
	providerLatency := time.Since(pStart)

	if err != nil {
		// Log error
		logRec := &logging.Record{
			Timestamp:      time.Now(),
			RequestID:      reqID,
			APIKeyID:       apiKeyRecord.ID.String(),
			APIKeyName:     apiKeyRecord.Name,
			Provider:       provider.Type(),
			Model:          providerModel,
			Alias:          modelName,
			ProviderMs:     providerLatency.Milliseconds(),
			GatewayMs:      time.Since(start).Milliseconds(),
			Error:          err.Error(),
			RequestPayload: payload,
		}
		_ = d.Logger.Enqueue(logRec)

		writeJSONError(w, http.StatusBadGateway, "provider error")
		return
	}

	// 10. Handle response based on streaming or non-streaming
	if isStreaming && pResp.Stream != nil {
		// Stream response to client
		d.handleStreamingResponse(w, r, pResp, apiKeyRecord, reqID, modelName, providerModel, provider, payload, start, providerLatency)
	} else {
		// Non-streaming response
		d.handleNonStreamingResponse(w, pResp, apiKeyRecord, reqID, modelName, providerModel, provider, payload, start, providerLatency)
	}
}

// handleNonStreamingResponse handles regular (non-streaming) provider responses
func (d *Dependencies) handleNonStreamingResponse(
	w http.ResponseWriter,
	pResp *providers.ChatResponse,
	apiKeyRecord *models.APIKey,
	reqID string,
	modelName string,
	providerModel string,
	provider providers.Provider,
	payload map[string]any,
	start time.Time,
	providerLatency time.Duration,
) {
	// Parse response to extract usage and cost
	var responseBody map[string]any
	if err := json.Unmarshal(pResp.Body, &responseBody); err == nil {
		// Successfully parsed response
	}

	// Create log record
	logRec := &logging.Record{
		Timestamp:       time.Now(),
		RequestID:       reqID,
		APIKeyID:        apiKeyRecord.ID.String(),
		APIKeyName:      apiKeyRecord.Name,
		Provider:        provider.Type(),
		Model:           providerModel,
		Alias:           modelName,
		ProviderMs:      providerLatency.Milliseconds(),
		GatewayMs:       time.Since(start).Milliseconds(),
		CostUSD:         pResp.CostUSD,
		RequestPayload:  payload,
		ResponsePayload: json.RawMessage(pResp.Body),
	}

	// Update billing (best-effort)
	if pResp.CostUSD > 0 {
		_ = d.Billing.AddUsage(context.Background(), apiKeyRecord.ID.String(), pResp.CostUSD)
	}

	// Enqueue log (best-effort)
	_ = d.Logger.Enqueue(logRec)

	// TODO: Record usage in database for detailed analytics
	// This would insert into usage_records table

	// Return provider response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(pResp.StatusCode)
	_, _ = w.Write(pResp.Body)
}

// handleStreamingResponse handles Server-Sent Events streaming from provider
func (d *Dependencies) handleStreamingResponse(
	w http.ResponseWriter,
	r *http.Request,
	pResp *providers.ChatResponse,
	apiKeyRecord *models.APIKey,
	reqID string,
	modelName string,
	providerModel string,
	provider providers.Provider,
	payload map[string]any,
	start time.Time,
	providerLatency time.Duration,
) {
	// Set headers for SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(pResp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	defer pResp.Stream.Close()

	// Stream events to client
	reader := providers.NewStreamReader(pResp.Stream)
	defer reader.Close()

	totalCost := 0.0
	eventCount := 0

	for {
		event, err := reader.Read()
		if err == io.EOF || (event != nil && event.Done) {
			break
		}
		if err != nil {
			// Error reading stream - log and break
			break
		}

		// Forward event to client
		if event.Data != nil {
			_, writeErr := w.Write([]byte("data: "))
			if writeErr != nil {
				break
			}
			_, writeErr = w.Write(event.Data)
			if writeErr != nil {
				break
			}
			_, writeErr = w.Write([]byte("\n\n"))
			if writeErr != nil {
				break
			}
			flusher.Flush()
			eventCount++
		}
	}

	// Send [DONE] marker
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()

	// Log the streaming request
	// Note: For streaming, cost calculation is more complex
	// We'd need to parse all chunks to get token counts
	logRec := &logging.Record{
		Timestamp:       time.Now(),
		RequestID:       reqID,
		APIKeyID:        apiKeyRecord.ID.String(),
		APIKeyName:      apiKeyRecord.Name,
		Provider:        provider.Type(),
		Model:           providerModel,
		Alias:           modelName,
		ProviderMs:      providerLatency.Milliseconds(),
		GatewayMs:       time.Since(start).Milliseconds(),
		CostUSD:         totalCost,
		RequestPayload:  payload,
		ResponsePayload: map[string]any{"stream": true, "events": eventCount},
	}

	_ = d.Logger.Enqueue(logRec)

	// Update billing if we have cost info
	if totalCost > 0 {
		_ = d.Billing.AddUsage(context.Background(), apiKeyRecord.ID.String(), totalCost)
	}
}

// parseBearer extracts the token from an Authorization: Bearer <token> header.
func parseBearer(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization header format")
	}
	if parts[1] == "" {
		return "", errors.New("empty bearer token")
	}
	return parts[1], nil
}

// newRequestID returns a UUID request ID for tracing
func newRequestID() string {
	return uuid.New().String()
}

// writeJSONError writes an OpenAI-compatible error response
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "invalid_request_error",
			"code":    statusCode,
		},
	}

	_ = json.NewEncoder(w).Encode(errorResp)
}
