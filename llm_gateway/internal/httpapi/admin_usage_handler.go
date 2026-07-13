package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

type AdminUsageHandler struct{ repository *storage.AdminUsageRepository }

func NewAdminUsageHandler(db *storage.DB) *AdminUsageHandler {
	return &AdminUsageHandler{repository: storage.NewAdminUsageRepository(db)}
}

func boundedPage(query mapQuery) (int, int) {
	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(query.Get("page_size"))
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

type mapQuery interface{ Get(string) string }

func optionalUUID(value string) (*uuid.UUID, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	return &parsed, err
}

func usageRange(q mapQuery, now time.Time) (time.Time, time.Time, error) {
	end := now.UTC()
	start := end.Add(-24 * time.Hour)
	var err error
	if value := q.Get("start"); value != "" {
		start, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if value := q.Get("end"); value != "" {
		end, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !end.After(start) || end.Sub(start) > 90*24*time.Hour {
		return time.Time{}, time.Time{}, strconv.ErrRange
	}
	return start.UTC(), end.UTC(), nil
}

func (h *AdminUsageHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	q := r.URL.Query()
	page, size := boundedPage(q)
	start, end, err := usageRange(q, time.Now())
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "start/end must be RFC3339, ordered, and no more than 90 days apart")
		return
	}
	keyID, err := optionalUUID(q.Get("api_key_id"))
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}
	var status *int
	if value := q.Get("status_code"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 100 || parsed > 599 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid status code")
			return
		}
		status = &parsed
	}
	result, err := h.repository.List(r.Context(), storage.AdminUsageFilters{Start: start, End: end, APIKeyID: keyID, ModelName: q.Get("model"), StatusCode: status, Page: page, PageSize: size})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list usage")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"items": result.Records, "total_count": result.TotalCount, "page": result.Page, "page_size": result.PageSize, "start": start, "end": end})
}

func (h *AdminUsageHandler) Monthly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	q := r.URL.Query()
	now := time.Now().UTC()
	year := now.Year()
	month := int(now.Month())
	page, size := boundedPage(q)
	if value := q.Get("year"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 2000 || parsed > 9999 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid year")
			return
		}
		year = parsed
	}
	if value := q.Get("month"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 12 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid month")
			return
		}
		month = parsed
	}
	keyID, err := optionalUUID(q.Get("api_key_id"))
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid API key ID")
		return
	}
	result, err := h.repository.ListMonthly(r.Context(), storage.AdminMonthlyFilters{Year: year, Month: month, APIKeyID: keyID, Page: page, PageSize: size})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list monthly billing")
		return
	}
	items := make([]map[string]interface{}, 0, len(result.Summaries))
	var totalCost int64
	var totalRequests, totalTokens int
	for _, s := range result.Summaries {
		cost := s.TotalCostNanoUSD.Float64()
		totalCost += int64(s.TotalCostNanoUSD)
		tokens := s.TotalInputTokens + s.TotalOutputTokens + s.TotalCachedTokens + s.TotalReasoningTokens
		totalRequests += s.TotalRequests
		totalTokens += tokens
		items = append(items, map[string]interface{}{"api_key_id": s.APIKeyID, "api_key_name": s.APIKeyName, "year": s.Year, "month": s.Month, "total_requests": s.TotalRequests, "total_input_tokens": s.TotalInputTokens, "total_output_tokens": s.TotalOutputTokens, "total_cached_tokens": s.TotalCachedTokens, "total_reasoning_tokens": s.TotalReasoningTokens, "total_tokens": tokens, "total_cost_usd": cost, "currency": "USD"})
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"items": items, "total_count": result.TotalCount, "page": result.Page, "page_size": result.PageSize, "year": year, "month": month, "page_totals": map[string]interface{}{"requests": totalRequests, "tokens": totalTokens, "cost_usd": float64(totalCost) / 1e9, "currency": "USD"}})
}

func (h *AdminUsageHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	hours := 24
	if value := r.URL.Query().Get("hours"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 168 {
			utils.RespondWithError(w, http.StatusBadRequest, "hours must be between 1 and 168")
			return
		}
		hours = parsed
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(hours) * time.Hour)
	summary, topModels, topKeys, err := h.repository.Dashboard(r.Context(), start, end)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to load dashboard")
		return
	}
	errorRate := 0.0
	if summary.Requests > 0 {
		errorRate = float64(summary.Errors) / float64(summary.Requests)
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"range": map[string]interface{}{"start": start, "end": end, "hours": hours}, "counts": map[string]int{"api_keys": summary.APIKeys, "models": summary.Models, "providers": summary.Providers}, "usage": map[string]interface{}{"requests": summary.Requests, "errors": summary.Errors, "error_rate": errorRate, "tokens": summary.Tokens, "average_latency_ms": summary.AverageLatencyMS}, "current_month": map[string]interface{}{"cost_usd": summary.CurrentMonthCostNanoUSD.Float64(), "currency": "USD"}, "top_models": topModels, "top_api_keys": topKeys})
}
