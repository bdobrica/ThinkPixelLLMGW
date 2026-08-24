package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/billing"
	"llm_gateway/internal/logging"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/models"
	"llm_gateway/internal/providers"
	"llm_gateway/internal/responses"
	"llm_gateway/internal/storage"
)

type nativeResponsesTransport struct{ provider providers.ResponsesProvider }

func (t nativeResponsesTransport) CreateResponse(ctx context.Context, payload []byte, stream bool) (*responses.TransportResponse, error) {
	result, err := t.provider.CreateResponse(ctx, providers.ResponsesRequest{Payload: payload, Stream: stream})
	if err != nil {
		return nil, err
	}
	return &responses.TransportResponse{StatusCode: result.StatusCode, Body: result.Body}, nil
}

func (d *Dependencies) handleResponseResource(w http.ResponseWriter, r *http.Request) {
	apiKey, ok := middleware.GetAPIKeyRecord(r.Context())
	if !ok {
		writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "missing API key context")
		return
	}
	owner, err := uuid.Parse(apiKey.ID)
	if err != nil {
		writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "invalid API key identity")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/responses/")
	if id == "" || strings.Contains(id, "/") || !strings.HasPrefix(id, "resp_") {
		writeResponsesError(w, http.StatusNotFound, nil, "not_found", "response not found")
		return
	}
	if d.Responses == nil {
		writeResponsesError(w, http.StatusServiceUnavailable, nil, "server_error", "Responses service unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if len(r.URL.Query()) != 0 {
			param := "include"
			writeResponsesError(w, http.StatusBadRequest, &param, "unsupported_parameter", "response retrieval options are not enabled")
			return
		}
		result, err := d.Responses.Retrieve(r.Context(), owner, id)
		if err != nil {
			writeResponseResourceError(w, err)
			return
		}
		writeResponsesJSON(w, http.StatusOK, result)
	case http.MethodDelete:
		result, err := d.Responses.Delete(r.Context(), owner, id)
		if err != nil {
			writeResponseResourceError(w, err)
			return
		}
		writeResponsesJSON(w, http.StatusOK, result)
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodDelete)
		writeResponsesError(w, http.StatusMethodNotAllowed, nil, "method_not_allowed", "method not allowed")
	}
}

func writeResponseResourceError(w http.ResponseWriter, err error) {
	if errors.Is(err, responses.ErrNotFound) {
		writeResponsesError(w, http.StatusNotFound, nil, "not_found", "response not found")
		return
	}
	writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "response state unavailable")
}

func writeResponsesJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (d *Dependencies) handleResponses(w http.ResponseWriter, r *http.Request) {
	start, requestID := time.Now(), newRequestID()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeResponsesError(w, http.StatusMethodNotAllowed, nil, "method_not_allowed", "method not allowed")
		return
	}
	apiKey, ok := middleware.GetAPIKeyRecord(r.Context())
	if !ok {
		writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "missing API key context")
		return
	}
	var raw json.RawMessage
	if err := decodeJSONBodyLimited(w, r, &raw); err != nil {
		status := http.StatusBadRequest
		if isRequestBodyTooLarge(err) {
			status = http.StatusRequestEntityTooLarge
		}
		writeResponsesError(w, status, nil, "invalid_json", "invalid Responses request")
		return
	}
	request, err := responses.DecodeCreateRequest(raw)
	if err != nil {
		writeResponsesValidationError(w, err)
		return
	}
	provider, providerModel, detailsValue, err := d.Providers.ResolveModelWithDetails(r.Context(), request.Model)
	if err != nil {
		param := "model"
		writeResponsesError(w, http.StatusBadRequest, &param, "model_not_found", "unknown model: "+request.Model)
		return
	}
	if !apiKey.AllowsModel(providerModel) {
		writeResponsesError(w, http.StatusForbidden, nil, "permission_denied", "API key not allowed to use this model")
		return
	}
	providerCaps, ok := responses.ProviderCapabilities(provider.Type())
	if !ok {
		writeResponsesUnsupported(w, "model", "selected provider does not support Responses")
		return
	}
	details, _ := detailsValue.(*storage.ModelWithDetails)
	if details == nil || details.Model == nil {
		writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "model details unavailable")
		return
	}
	modelCaps := responses.ResolveModelCapabilities(providerCaps, details.SupportsReasoning, details.SupportsFunctionCalling,
		details.SupportsParallelFunctionCalling, details.SupportsNativeStreaming || details.SupportsStreamingOutput, details.SupportsWebSearch)
	if err := responses.ValidateCapabilities(*request, modelCaps); err != nil {
		writeResponsesValidationError(w, err)
		return
	}
	nativeProvider, ok := provider.(providers.ResponsesProvider)
	if !ok || providerCaps.ResponsesTransport != responses.SupportNative {
		writeResponsesUnsupported(w, "model", "item-aware translation is not enabled for the selected provider")
		return
	}
	allowed, remaining, resetAt, err := d.RateLimit.AllowWithDetails(r.Context(), apiKey.ID, apiKey.RateLimitPerMinute)
	if err != nil {
		writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "rate limit check error")
		return
	}
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", apiKey.RateLimitPerMinute))
	if apiKey.RateLimitPerMinute > 0 {
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
	}
	if !allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", max(0, int(time.Until(resetAt).Seconds()))))
		writeResponsesError(w, http.StatusTooManyRequests, nil, "rate_limit_exceeded", "rate limit exceeded")
		return
	}
	if !d.Billing.WithinBudget(r.Context(), apiKey.ID) {
		writeResponsesError(w, http.StatusPaymentRequired, nil, "budget_exceeded", "monthly budget exceeded")
		return
	}
	owner, err := uuid.Parse(apiKey.ID)
	if err != nil {
		writeResponsesError(w, http.StatusInternalServerError, nil, "server_error", "invalid API key identity")
		return
	}
	result, status, err := d.Responses.CreateNative(r.Context(), *request, responses.CreateOptions{Owner: owner, ProviderModel: providerModel,
		Transport: nativeResponsesTransport{provider: nativeProvider}})
	if err != nil {
		var upstream *responses.UpstreamError
		if errors.As(err, &upstream) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(upstream.StatusCode)
			_, _ = w.Write(upstream.Body)
			return
		}
		if errors.Is(err, responses.ErrNotFound) {
			writeResponsesError(w, http.StatusNotFound, nil, "not_found", "response not found")
			return
		}
		if _, ok := err.(*responses.ValidationError); ok {
			writeResponsesValidationError(w, err)
			return
		}
		writeResponsesError(w, http.StatusBadGateway, nil, "provider_error", "provider error")
		return
	}
	usage := result.Usage
	actualCost := 0.0
	if usage != nil {
		actualCost = details.CalculateCost(models.UsageRecord{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
			CachedTokens: usage.InputTokensDetails.CachedTokens, ReasoningTokens: usage.OutputTokensDetails.ReasoningTokens})
	}
	d.enqueueLogRecord(&logging.LogRecord{Timestamp: time.Now(), RequestID: requestID, APIKeyID: apiKey.ID, APIKeyName: apiKey.Name,
		Provider: provider.Type(), Model: providerModel, Alias: request.Model, ProviderMs: time.Since(start).Milliseconds(),
		GatewayMs: time.Since(start).Milliseconds(), CostUSD: actualCost, RequestPayload: map[string]any{"model": request.Model, "response_id": result.ID}})
	if actualCost > 0 && d.BillingWorker != nil {
		d.enqueueBillingUpdate(requestID, &billing.BillingUpdate{APIKeyID: apiKey.ID, CostUSD: actualCost, Timestamp: time.Now()})
	}
	if usage != nil && d.UsageWorker != nil {
		apiKeyID, requestUUID, parseErr := parseUsageRecordIDs(apiKey.ID, requestID)
		if parseErr == nil {
			d.enqueueUsageRecord(requestID, &models.UsageRecord{ID: uuid.New(), APIKeyID: apiKeyID, RequestID: requestUUID,
				ModelName: request.Model, Endpoint: "/v1/responses", InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
				CachedTokens: usage.InputTokensDetails.CachedTokens, ReasoningTokens: usage.OutputTokensDetails.ReasoningTokens,
				ResponseTimeMS: int(time.Since(start).Milliseconds()), StatusCode: status})
		}
		d.Metrics.RecordRequest(apiKey.ID, apiKey.Name, apiKey.Tags, usage.InputTokens, usage.InputTokensDetails.CachedTokens,
			usage.OutputTokens, actualCost, time.Since(start))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}

func writeResponsesValidationError(w http.ResponseWriter, err error) {
	var validation *responses.ValidationError
	if errors.As(err, &validation) {
		param := validation.Param
		writeResponsesError(w, http.StatusBadRequest, &param, validation.Code, validation.Message)
		return
	}
	writeResponsesError(w, http.StatusBadRequest, nil, "invalid_request", err.Error())
}
func writeResponsesUnsupported(w http.ResponseWriter, param, message string) {
	writeResponsesError(w, http.StatusBadRequest, &param, "unsupported_endpoint", message)
}
func writeResponsesError(w http.ResponseWriter, status int, param *string, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responses.ErrorEnvelope{Error: responses.APIError{Message: message, Type: "invalid_request_error", Param: param, Code: code}})
}
