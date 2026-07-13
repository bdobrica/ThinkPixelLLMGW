package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
)

type AdminUsageFilters struct {
	Start, End     time.Time
	APIKeyID       *uuid.UUID
	ModelName      string
	StatusCode     *int
	Page, PageSize int
}

type AdminUsageRecord struct {
	ID              uuid.UUID `db:"id" json:"id"`
	APIKeyID        uuid.UUID `db:"api_key_id" json:"api_key_id"`
	APIKeyName      string    `db:"api_key_name" json:"api_key_name"`
	RequestID       uuid.UUID `db:"request_id" json:"request_id"`
	ModelName       string    `db:"model_name" json:"model_name"`
	Endpoint        string    `db:"endpoint" json:"endpoint"`
	InputTokens     int       `db:"input_tokens" json:"input_tokens"`
	OutputTokens    int       `db:"output_tokens" json:"output_tokens"`
	CachedTokens    int       `db:"cached_tokens" json:"cached_tokens"`
	ReasoningTokens int       `db:"reasoning_tokens" json:"reasoning_tokens"`
	ResponseTimeMS  int       `db:"response_time_ms" json:"response_time_ms"`
	StatusCode      int       `db:"status_code" json:"status_code"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

type AdminUsageResult struct {
	Records    []AdminUsageRecord
	TotalCount int
	Page       int
	PageSize   int
}

type AdminMonthlyFilters struct {
	Year, Month    int
	APIKeyID       *uuid.UUID
	Page, PageSize int
}
type AdminMonthlySummary struct {
	APIKeyID             uuid.UUID      `db:"api_key_id"`
	APIKeyName           string         `db:"api_key_name"`
	Year                 int            `db:"year"`
	Month                int            `db:"month"`
	TotalRequests        int            `db:"total_requests"`
	TotalInputTokens     int            `db:"total_input_tokens"`
	TotalOutputTokens    int            `db:"total_output_tokens"`
	TotalCachedTokens    int            `db:"total_cached_tokens"`
	TotalReasoningTokens int            `db:"total_reasoning_tokens"`
	TotalCostNanoUSD     models.NanoUSD `db:"total_cost_nano_usd"`
}
type AdminMonthlyResult struct {
	Summaries  []AdminMonthlySummary
	TotalCount int
	Page       int
	PageSize   int
}

type AdminUsageRepository struct{ db *DB }

func NewAdminUsageRepository(db *DB) *AdminUsageRepository { return &AdminUsageRepository{db: db} }

func (r *AdminUsageRepository) List(ctx context.Context, f AdminUsageFilters) (*AdminUsageResult, error) {
	where := []string{"u.created_at >= $1", "u.created_at < $2"}
	args := []interface{}{f.Start, f.End}
	n := 3
	if f.APIKeyID != nil {
		where = append(where, fmt.Sprintf("u.api_key_id = $%d", n))
		args = append(args, *f.APIKeyID)
		n++
	}
	if f.ModelName != "" {
		where = append(where, fmt.Sprintf("u.model_name = $%d", n))
		args = append(args, f.ModelName)
		n++
	}
	if f.StatusCode != nil {
		where = append(where, fmt.Sprintf("u.status_code = $%d", n))
		args = append(args, *f.StatusCode)
		n++
	}
	clause := strings.Join(where, " AND ")
	var count int
	if err := r.db.conn.GetContext(ctx, &count, "SELECT COUNT(*) FROM usage_records u WHERE "+clause, args...); err != nil {
		return nil, fmt.Errorf("failed to count usage: %w", err)
	}
	offset := (f.Page - 1) * f.PageSize
	query := fmt.Sprintf(`SELECT u.id,u.api_key_id,k.name api_key_name,u.request_id,u.model_name,u.endpoint,u.input_tokens,u.output_tokens,u.cached_tokens,u.reasoning_tokens,u.response_time_ms,u.status_code,u.created_at FROM usage_records u JOIN api_keys k ON k.id=u.api_key_id WHERE %s ORDER BY u.created_at DESC,u.id DESC LIMIT $%d OFFSET $%d`, clause, n, n+1)
	args = append(args, f.PageSize, offset)
	records := []AdminUsageRecord{}
	if err := r.db.conn.SelectContext(ctx, &records, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list usage: %w", err)
	}
	return &AdminUsageResult{Records: records, TotalCount: count, Page: f.Page, PageSize: f.PageSize}, nil
}

func (r *AdminUsageRepository) ListMonthly(ctx context.Context, f AdminMonthlyFilters) (*AdminMonthlyResult, error) {
	where := []string{"m.year = $1", "m.month = $2"}
	args := []interface{}{f.Year, f.Month}
	n := 3
	if f.APIKeyID != nil {
		where = append(where, fmt.Sprintf("m.api_key_id = $%d", n))
		args = append(args, *f.APIKeyID)
		n++
	}
	clause := strings.Join(where, " AND ")
	var count int
	if err := r.db.conn.GetContext(ctx, &count, "SELECT COUNT(*) FROM monthly_usage_summary m WHERE "+clause, args...); err != nil {
		return nil, fmt.Errorf("failed to count monthly usage: %w", err)
	}
	query := fmt.Sprintf(`SELECT m.api_key_id,k.name api_key_name,m.year,m.month,m.total_requests,m.total_input_tokens,m.total_output_tokens,m.total_cached_tokens,m.total_reasoning_tokens,m.total_cost_nano_usd FROM monthly_usage_summary m JOIN api_keys k ON k.id=m.api_key_id WHERE %s ORDER BY m.total_cost_nano_usd DESC,m.api_key_id LIMIT $%d OFFSET $%d`, clause, n, n+1)
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	summaries := []AdminMonthlySummary{}
	if err := r.db.conn.SelectContext(ctx, &summaries, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list monthly usage: %w", err)
	}
	return &AdminMonthlyResult{Summaries: summaries, TotalCount: count, Page: f.Page, PageSize: f.PageSize}, nil
}
