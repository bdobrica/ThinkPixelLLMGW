package storage

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	
	"gateway/internal/models"
)

// UsageRepository handles usage record database operations
type UsageRepository struct {
	db *DB
}

// NewUsageRepository creates a new usage repository
func NewUsageRepository(db *DB) *UsageRepository {
	return &UsageRepository{db: db}
}

// Create creates a new usage record
func (r *UsageRepository) Create(ctx context.Context, record *models.UsageRecord) error {
	query := `
		INSERT INTO usage_records (
			id, api_key_id, model_id, provider_id, request_timestamp,
			prompt_tokens, completion_tokens, total_tokens, cost_usd,
			request_metadata, response_metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at
	`
	
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	
	if record.RequestTimestamp.IsZero() {
		record.RequestTimestamp = time.Now()
	}
	
	err := r.db.conn.QueryRowxContext(
		ctx, query,
		record.ID, record.APIKeyID, record.ModelID, record.ProviderID,
		record.RequestTimestamp, record.PromptTokens, record.CompletionTokens,
		record.TotalTokens, record.CostUSD, record.RequestMetadata,
		record.ResponseMetadata,
	).Scan(&record.CreatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create usage record: %w", err)
	}
	
	return nil
}

// GetByAPIKey retrieves usage records for an API key
func (r *UsageRepository) GetByAPIKey(ctx context.Context, apiKeyID uuid.UUID, startTime, endTime time.Time, limit, offset int) ([]*models.UsageRecord, error) {
	query := `
		SELECT id, api_key_id, model_id, provider_id, request_timestamp,
		       prompt_tokens, completion_tokens, total_tokens, cost_usd,
		       request_metadata, response_metadata, created_at
		FROM usage_records
		WHERE api_key_id = $1 
		  AND request_timestamp >= $2 
		  AND request_timestamp < $3
		ORDER BY request_timestamp DESC
		LIMIT $4 OFFSET $5
	`
	
	var records []*models.UsageRecord
	err := r.db.conn.SelectContext(ctx, &records, query, apiKeyID, startTime, endTime, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage records: %w", err)
	}
	
	return records, nil
}

// GetByModel retrieves usage records for a model
func (r *UsageRepository) GetByModel(ctx context.Context, modelID uuid.UUID, startTime, endTime time.Time, limit, offset int) ([]*models.UsageRecord, error) {
	query := `
		SELECT id, api_key_id, model_id, provider_id, request_timestamp,
		       prompt_tokens, completion_tokens, total_tokens, cost_usd,
		       request_metadata, response_metadata, created_at
		FROM usage_records
		WHERE model_id = $1 
		  AND request_timestamp >= $2 
		  AND request_timestamp < $3
		ORDER BY request_timestamp DESC
		LIMIT $4 OFFSET $5
	`
	
	var records []*models.UsageRecord
	err := r.db.conn.SelectContext(ctx, &records, query, modelID, startTime, endTime, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage records: %w", err)
	}
	
	return records, nil
}

// GetTotalCostByAPIKey calculates total cost for an API key in a time range
func (r *UsageRepository) GetTotalCostByAPIKey(ctx context.Context, apiKeyID uuid.UUID, startTime, endTime time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(cost_usd), 0)
		FROM usage_records
		WHERE api_key_id = $1 
		  AND request_timestamp >= $2 
		  AND request_timestamp < $3
	`
	
	var totalCost float64
	err := r.db.conn.GetContext(ctx, &totalCost, query, apiKeyID, startTime, endTime)
	if err != nil {
		return 0, fmt.Errorf("failed to get total cost: %w", err)
	}
	
	return totalCost, nil
}

// GetTotalTokensByAPIKey calculates total tokens for an API key in a time range
func (r *UsageRepository) GetTotalTokensByAPIKey(ctx context.Context, apiKeyID uuid.UUID, startTime, endTime time.Time) (int, int, int, error) {
	query := `
		SELECT 
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0)
		FROM usage_records
		WHERE api_key_id = $1 
		  AND request_timestamp >= $2 
		  AND request_timestamp < $3
	`
	
	var promptTokens, completionTokens, totalTokens int
	err := r.db.conn.QueryRowxContext(ctx, query, apiKeyID, startTime, endTime).
		Scan(&promptTokens, &completionTokens, &totalTokens)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get total tokens: %w", err)
	}
	
	return promptTokens, completionTokens, totalTokens, nil
}

// MonthlyUsageSummaryRepository handles monthly usage summary operations
type MonthlyUsageSummaryRepository struct {
	db *DB
}

// NewMonthlyUsageSummaryRepository creates a new monthly usage summary repository
func NewMonthlyUsageSummaryRepository(db *DB) *MonthlyUsageSummaryRepository {
	return &MonthlyUsageSummaryRepository{db: db}
}

// GetByAPIKeyAndMonth retrieves monthly usage summary for an API key
func (r *MonthlyUsageSummaryRepository) GetByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) (*models.MonthlyUsageSummary, error) {
	query := `
		SELECT id, api_key_id, year, month, total_requests, total_prompt_tokens,
		       total_completion_tokens, total_tokens, total_cost_usd, created_at, updated_at
		FROM monthly_usage_summary
		WHERE api_key_id = $1 AND year = $2 AND month = $3
	`
	
	var summary models.MonthlyUsageSummary
	err := r.db.conn.GetContext(ctx, &summary, query, apiKeyID, year, month)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly usage summary: %w", err)
	}
	
	return &summary, nil
}

// Upsert creates or updates monthly usage summary
func (r *MonthlyUsageSummaryRepository) Upsert(ctx context.Context, summary *models.MonthlyUsageSummary) error {
	query := `
		INSERT INTO monthly_usage_summary (
			id, api_key_id, year, month, total_requests, total_prompt_tokens,
			total_completion_tokens, total_tokens, total_cost_usd
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (api_key_id, year, month)
		DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			total_prompt_tokens = EXCLUDED.total_prompt_tokens,
			total_completion_tokens = EXCLUDED.total_completion_tokens,
			total_tokens = EXCLUDED.total_tokens,
			total_cost_usd = EXCLUDED.total_cost_usd,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`
	
	if summary.ID == uuid.Nil {
		summary.ID = uuid.New()
	}
	
	err := r.db.conn.QueryRowxContext(
		ctx, query,
		summary.ID, summary.APIKeyID, summary.Year, summary.Month,
		summary.TotalRequests, summary.TotalPromptTokens, summary.TotalCompletionTokens,
		summary.TotalTokens, summary.TotalCostUSD,
	).Scan(&summary.CreatedAt, &summary.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to upsert monthly usage summary: %w", err)
	}
	
	return nil
}

// GetByAPIKey retrieves all monthly summaries for an API key
func (r *MonthlyUsageSummaryRepository) GetByAPIKey(ctx context.Context, apiKeyID uuid.UUID, limit, offset int) ([]*models.MonthlyUsageSummary, error) {
	query := `
		SELECT id, api_key_id, year, month, total_requests, total_prompt_tokens,
		       total_completion_tokens, total_tokens, total_cost_usd, created_at, updated_at
		FROM monthly_usage_summary
		WHERE api_key_id = $1
		ORDER BY year DESC, month DESC
		LIMIT $2 OFFSET $3
	`
	
	var summaries []*models.MonthlyUsageSummary
	err := r.db.conn.SelectContext(ctx, &summaries, query, apiKeyID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly usage summaries: %w", err)
	}
	
	return summaries, nil
}

// RefreshSummary recalculates monthly summary from usage records
func (r *MonthlyUsageSummaryRepository) RefreshSummary(ctx context.Context, apiKeyID uuid.UUID, year, month int) error {
	query := `
		INSERT INTO monthly_usage_summary (
			id, api_key_id, year, month, total_requests, total_prompt_tokens,
			total_completion_tokens, total_tokens, total_cost_usd
		)
		SELECT 
			gen_random_uuid(),
			$1,
			$2,
			$3,
			COUNT(*),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(cost_usd), 0)
		FROM usage_records
		WHERE api_key_id = $1
		  AND EXTRACT(YEAR FROM request_timestamp) = $2
		  AND EXTRACT(MONTH FROM request_timestamp) = $3
		ON CONFLICT (api_key_id, year, month)
		DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			total_prompt_tokens = EXCLUDED.total_prompt_tokens,
			total_completion_tokens = EXCLUDED.total_completion_tokens,
			total_tokens = EXCLUDED.total_tokens,
			total_cost_usd = EXCLUDED.total_cost_usd,
			updated_at = NOW()
	`
	
	_, err := r.db.conn.ExecContext(ctx, query, apiKeyID, year, month)
	if err != nil {
		return fmt.Errorf("failed to refresh monthly usage summary: %w", err)
	}
	
	return nil
}
