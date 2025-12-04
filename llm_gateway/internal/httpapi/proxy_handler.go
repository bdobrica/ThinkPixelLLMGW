package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/billing"
	"llm_gateway/internal/logging"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/storage"
)

// handleChat is the entry point for OpenAI-compatible chat completions.
// This handler is protected by APIKeyMiddleware, so the API key has already been validated.
//
// Flow:
//  1. Validate method
//  2. Get authenticated API key from context (set by middleware)
//  3. Decode JSON body
//  4. Resolve model/alias → provider + actual model name + model details
//  5. Check key permissions (against resolved model name)
//  6. Rate limit
//  7. Budget check
//  8. Call provider
//  9. Log + update billing
// 10. Return provider response
func (d *Dependencies) handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := newRequestID()

	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	// 1. Get API key record from context (set by APIKeyMiddleware)
	apiKeyRecord, ok := middleware.GetAPIKeyRecord(ctx)
	if !ok {
		// This should never happen if middleware is properly applied
		writeJSONError(w, http.StatusInternalServerError, "internal error: missing API key context")
		return
	}

	// 2. Decode request body as generic JSON (OpenAI-style payload).
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// 3. Extract model name.
	modelName, _ := payload["model"].(string)
	if modelName == "" {
		writeJSONError(w, http.StatusBadRequest, "missing 'model' field")
		return
	}

	// Check if streaming is requested
	isStreaming, _ := payload["stream"].(bool)

	// 4. Resolve model → provider + providerModel + model details (with pricing)
	// This also resolves aliases to actual model names
	provider, providerModel, modelDetails, err := d.Providers.ResolveModelWithDetails(ctx, modelName)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("unknown model: %s", modelName))
		return
	}

	// 5. Check if key is allowed to call this model (use the resolved model name)
	if !apiKeyRecord.AllowsModel(providerModel) {
		writeJSONError(w, http.StatusForbidden, "API key not allowed to use this model")
		return
	}

	// 6. Rate limit check with detailed information
	allowed, remaining, resetAt, err := d.RateLimit.AllowWithDetails(ctx, apiKeyRecord.ID, apiKeyRecord.RateLimitPerMinute)
	if err != nil {
		// Log the error but don't fail the request - fallback to allowing
		// TODO: Add proper error logging
		writeJSONError(w, http.StatusInternalServerError, "rate limit check error")
		return
	}

	// Set rate limit headers
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", apiKeyRecord.RateLimitPerMinute))
	if apiKeyRecord.RateLimitPerMinute > 0 {
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
	}

	if !allowed {
		// Add Retry-After header (seconds until reset)
		retryAfter := int(time.Until(resetAt).Seconds())
		if retryAfter < 0 {
			retryAfter = 60 // Default to 60 seconds
		}
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	// 6. Budget check
	withinBudget := d.Billing.WithinBudget(ctx, apiKeyRecord.ID)
	if !withinBudget {
		writeJSONError(w, http.StatusPaymentRequired, "monthly budget exceeded")
		return
	}

	// 7. Call provider
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
		logRec := &logging.LogRecord{
			Timestamp:      time.Now(),
			RequestID:      reqID,
			APIKeyID:       apiKeyRecord.ID,
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
		d.handleNonStreamingResponse(w, pResp, apiKeyRecord, reqID, modelName, providerModel, provider, payload, start, providerLatency, modelDetails)
	}
}

// handleNonStreamingResponse handles regular (non-streaming) provider responses
func (d *Dependencies) handleNonStreamingResponse(
	w http.ResponseWriter,
	pResp *providers.ChatResponse,
	apiKeyRecord *auth.APIKeyRecord,
	reqID string,
	modelName string,
	providerModel string,
	provider providers.Provider,
	payload map[string]any,
	start time.Time,
	providerLatency time.Duration,
	modelDetails interface{},
) {
	// Parse response to extract usage and cost
	var responseBody map[string]any
	if err := json.Unmarshal(pResp.Body, &responseBody); err == nil {
		// Successfully parsed response
	}

	// Calculate accurate cost using model pricing components
	actualCost := pResp.CostUSD // Use provider's fallback calculation
	if modelDetails != nil {
		// Type assert to get the actual model with pricing components
		if details, ok := modelDetails.(*storage.ModelWithDetails); ok && details.Model != nil {
			// Create usage record from response
			usageRecord := models.UsageRecord{
				InputTokens:     pResp.InputTokens,
				OutputTokens:    pResp.OutputTokens,
				CachedTokens:    pResp.CachedTokens,
				ReasoningTokens: pResp.ReasoningTokens,
			}

			// Calculate cost using model's pricing components
			actualCost = details.Model.CalculateCost(usageRecord)
		}
	}

	// Create log record
	logRec := &logging.LogRecord{
		Timestamp:       time.Now(),
		RequestID:       reqID,
		APIKeyID:        apiKeyRecord.ID,
		APIKeyName:      apiKeyRecord.Name,
		Provider:        provider.Type(),
		Model:           providerModel,
		Alias:           modelName,
		ProviderMs:      providerLatency.Milliseconds(),
		GatewayMs:       time.Since(start).Milliseconds(),
		CostUSD:         actualCost,
		RequestPayload:  payload,
		ResponsePayload: json.RawMessage(pResp.Body),
	}

	// Enqueue log (best-effort)
	_ = d.Logger.Enqueue(logRec)

	// Queue billing update asynchronously
	if actualCost > 0 && d.BillingWorker != nil {
		billingUpdate := &billing.BillingUpdate{
			APIKeyID:  apiKeyRecord.ID,
			CostUSD:   actualCost,
			Timestamp: time.Now(),
		}
		_ = d.BillingWorker.Enqueue(context.Background(), billingUpdate)
	}

	// Queue usage record asynchronously
	if d.UsageWorker != nil {
		usageRecord := &models.UsageRecord{
			ID:              uuid.New(),
			APIKeyID:        uuid.MustParse(apiKeyRecord.ID),
			RequestID:       uuid.MustParse(reqID),
			ModelName:       modelName,
			Endpoint:        "/v1/chat/completions",
			InputTokens:     pResp.InputTokens,
			OutputTokens:    pResp.OutputTokens,
			CachedTokens:    pResp.CachedTokens,
			ReasoningTokens: pResp.ReasoningTokens,
			ResponseTimeMS:  int(providerLatency.Milliseconds()),
			StatusCode:      pResp.StatusCode,
		}
		_ = d.UsageWorker.Enqueue(context.Background(), usageRecord)
	}

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
	apiKeyRecord *auth.APIKeyRecord,
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
	logRec := &logging.LogRecord{
		Timestamp:       time.Now(),
		RequestID:       reqID,
		APIKeyID:        apiKeyRecord.ID,
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

	// Queue billing update asynchronously
	if totalCost > 0 && d.BillingWorker != nil {
		billingUpdate := &billing.BillingUpdate{
			APIKeyID:  apiKeyRecord.ID,
			CostUSD:   totalCost,
			Timestamp: time.Now(),
		}
		_ = d.BillingWorker.Enqueue(context.Background(), billingUpdate)
	}

	// Note: For streaming responses, we don't have detailed token counts
	// unless we parse all chunks. This is a limitation of streaming.
	// Consider adding token counting from parsed chunks if needed.
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
