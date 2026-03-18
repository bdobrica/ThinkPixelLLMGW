package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
	"llm_gateway/internal/utils"
)

var proxyLogger = utils.NewLogger("proxy-handler", utils.Info)

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

	// 1. Get API key record from context (set by APIKeyMiddleware)
	apiKeyRecord, ok := middleware.GetAPIKeyRecord(ctx)
	if !ok {
		// This should never happen if middleware is properly applied
		writeJSONError(w, http.StatusInternalServerError, "internal error: missing API key context")
		return
	}

	// 2. Decode request body as generic JSON (OpenAI-style payload).
	var payload map[string]any
	if err := decodeJSONBodyLimited(w, r, &payload); err != nil {
		if isRequestBodyTooLarge(err) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
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
	if isStreaming && provider.Type() == "openai" {
		ensureStreamUsageInPayload(payload)
	}

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
		d.handleStreamingResponse(w, r, pResp, apiKeyRecord, reqID, modelName, providerModel, provider, payload, start, providerLatency, modelDetails)
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
		apiKeyUUID, requestUUID, err := parseUsageRecordIDs(apiKeyRecord.ID, reqID)
		if err != nil {
			proxyLogger.Warn("skipping usage record enqueue due to invalid IDs", "error", err, "api_key_id", apiKeyRecord.ID, "request_id", reqID)
		} else {
			usageRecord := &models.UsageRecord{
				ID:              uuid.New(),
				APIKeyID:        apiKeyUUID,
				RequestID:       requestUUID,
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
	}

	// Record metrics
	d.Metrics.RecordRequest(
		apiKeyRecord.ID,
		apiKeyRecord.Name,
		apiKeyRecord.Tags,
		pResp.InputTokens,
		pResp.CachedTokens,
		pResp.OutputTokens,
		actualCost,
		time.Since(start),
	)

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
	modelDetails interface{},
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Keep long-lived SSE responses open beyond the server's global write timeout.
	responseController := http.NewResponseController(w)
	if err := responseController.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		writeJSONError(w, http.StatusInternalServerError, "failed to initialize streaming response")
		return
	}

	// Set headers for SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(pResp.StatusCode)

	defer pResp.Stream.Close()

	// Stream events to client
	reader := providers.NewStreamReader(pResp.Stream)
	defer reader.Close()

	totalCost := 0.0
	inputTokens := 0
	outputTokens := 0
	cachedTokens := 0
	reasoningTokens := 0
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
			if usage, ok := extractStreamUsageFromEvent(event.Data); ok {
				inputTokens = usage.InputTokens
				outputTokens = usage.OutputTokens
				cachedTokens = usage.CachedTokens
				reasoningTokens = usage.ReasoningTokens
			}

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

	if modelDetails != nil {
		if details, ok := modelDetails.(*storage.ModelWithDetails); ok && details.Model != nil {
			usageRecord := models.UsageRecord{
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				CachedTokens:    cachedTokens,
				ReasoningTokens: reasoningTokens,
			}
			totalCost = details.Model.CalculateCost(usageRecord)
		}
	}
	if totalCost == 0 && (inputTokens > 0 || outputTokens > 0) {
		// Fallback estimate used only when model pricing is unavailable.
		totalCost = fallbackCostFromUsage(inputTokens, outputTokens)
	}

	// Log the streaming request
	// Note: For streaming, cost calculation is more complex
	// We'd need to parse all chunks to get token counts
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
		CostUSD:        totalCost,
		RequestPayload: payload,
		ResponsePayload: map[string]any{
			"stream":           true,
			"events":           eventCount,
			"input_tokens":     inputTokens,
			"output_tokens":    outputTokens,
			"cached_tokens":    cachedTokens,
			"reasoning_tokens": reasoningTokens,
		},
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

	if d.UsageWorker != nil {
		apiKeyUUID, requestUUID, err := parseUsageRecordIDs(apiKeyRecord.ID, reqID)
		if err != nil {
			proxyLogger.Warn("skipping streaming usage record enqueue due to invalid IDs", "error", err, "api_key_id", apiKeyRecord.ID, "request_id", reqID)
		} else {
			usageRecord := &models.UsageRecord{
				ID:              uuid.New(),
				APIKeyID:        apiKeyUUID,
				RequestID:       requestUUID,
				ModelName:       modelName,
				Endpoint:        "/v1/chat/completions",
				InputTokens:     inputTokens,
				OutputTokens:    outputTokens,
				CachedTokens:    cachedTokens,
				ReasoningTokens: reasoningTokens,
				ResponseTimeMS:  int(providerLatency.Milliseconds()),
				StatusCode:      pResp.StatusCode,
			}
			_ = d.UsageWorker.Enqueue(context.Background(), usageRecord)
		}
	}

	// Record metrics with extracted token usage when available.
	d.Metrics.RecordRequest(
		apiKeyRecord.ID,
		apiKeyRecord.Name,
		apiKeyRecord.Tags,
		inputTokens,
		cachedTokens,
		outputTokens,
		totalCost,
		time.Since(start),
	)
}

// ensureStreamUsageInPayload requests usage in final streaming chunk for OpenAI-compatible APIs.
func ensureStreamUsageInPayload(payload map[string]any) {
	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok || streamOptions == nil {
		streamOptions = map[string]any{}
	}
	streamOptions["include_usage"] = true
	payload["stream_options"] = streamOptions
}

// extractStreamUsageFromEvent extracts usage info from one SSE data chunk.
func extractStreamUsageFromEvent(data []byte) (providers.UsageInfo, bool) {
	type usageFields struct {
		InputTokens      int `json:"input_tokens"`
		OutputTokens     int `json:"output_tokens"`
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		PromptDetails    struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		InputDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		CompletionDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
		OutputDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
	}

	var chunk struct {
		Usage *usageFields `json:"usage"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil || chunk.Usage == nil {
		return providers.UsageInfo{}, false
	}

	usage := providers.UsageInfo{
		InputTokens:     chunk.Usage.InputTokens,
		OutputTokens:    chunk.Usage.OutputTokens,
		CachedTokens:    chunk.Usage.InputDetails.CachedTokens,
		ReasoningTokens: chunk.Usage.OutputDetails.ReasoningTokens,
	}

	if usage.InputTokens == 0 {
		usage.InputTokens = chunk.Usage.PromptTokens
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = chunk.Usage.CompletionTokens
	}
	if usage.CachedTokens == 0 {
		usage.CachedTokens = chunk.Usage.PromptDetails.CachedTokens
	}
	if usage.ReasoningTokens == 0 {
		usage.ReasoningTokens = chunk.Usage.CompletionDetails.ReasoningTokens
	}

	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CachedTokens == 0 && usage.ReasoningTokens == 0 {
		return providers.UsageInfo{}, false
	}

	return usage, true
}

func fallbackCostFromUsage(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) * 0.00001
	outputCost := float64(outputTokens) * 0.00003
	return inputCost + outputCost
}

func parseUsageRecordIDs(apiKeyID, requestID string) (uuid.UUID, uuid.UUID, error) {
	apiKeyUUID, err := uuid.Parse(apiKeyID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid API key UUID %q: %w", apiKeyID, err)
	}

	requestUUID, err := uuid.Parse(requestID)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid request UUID %q: %w", requestID, err)
	}

	return apiKeyUUID, requestUUID, nil
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
