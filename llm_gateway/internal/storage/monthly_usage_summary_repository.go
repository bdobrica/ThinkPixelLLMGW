package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
)

// MonthlyUsageSummaryRepository handles persisted monthly usage summary operations.
type MonthlyUsageSummaryRepository struct {
	db *DB
}

// NewMonthlyUsageSummaryRepository creates a new monthly usage summary repository.
func NewMonthlyUsageSummaryRepository(db *DB) *MonthlyUsageSummaryRepository {
	return &MonthlyUsageSummaryRepository{db: db}
}

// GetByAPIKeyAndMonth retrieves the summary row for a specific API key and month.
func (r *MonthlyUsageSummaryRepository) GetByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) (*models.MonthlyUsageSummary, error) {
	query := `
		SELECT id, api_key_id, year, month, total_requests, total_input_tokens,
		       total_output_tokens, total_cached_tokens, total_reasoning_tokens,
		       total_cost_usd, created_at, updated_at
		FROM monthly_usage_summary
		WHERE api_key_id = $1 AND year = $2 AND month = $3
	`

	var summary models.MonthlyUsageSummary
	err := r.db.conn.GetContext(ctx, &summary, query, apiKeyID, year, month)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrMonthlyUsageSummaryNotFound
		}
		return nil, fmt.Errorf("failed to get monthly usage summary: %w", err)
	}

	return &summary, nil
}

// UpsertCost creates or updates the persisted monthly total cost without disturbing token counters.
func (r *MonthlyUsageSummaryRepository) UpsertCost(ctx context.Context, apiKeyID uuid.UUID, year, month int, totalCostUSD float64) error {
	query := `
		INSERT INTO monthly_usage_summary (
			id, api_key_id, year, month, total_requests, total_input_tokens,
			total_output_tokens, total_cached_tokens, total_reasoning_tokens, total_cost_usd
		) VALUES ($1, $2, $3, $4, 0, 0, 0, 0, 0, $5)
		ON CONFLICT (api_key_id, year, month)
		DO UPDATE SET
			total_cost_usd = EXCLUDED.total_cost_usd,
			updated_at = NOW()
	`

	_, err := r.db.conn.ExecContext(ctx, query, uuid.New(), apiKeyID, year, month, totalCostUSD)
	if err != nil {
		return fmt.Errorf("failed to upsert monthly usage summary: %w", err)
	}

	return nil
}

// DeleteByAPIKeyAndMonth removes the persisted summary row for a specific API key and month.
func (r *MonthlyUsageSummaryRepository) DeleteByAPIKeyAndMonth(ctx context.Context, apiKeyID uuid.UUID, year, month int) error {
	query := `
		DELETE FROM monthly_usage_summary
		WHERE api_key_id = $1 AND year = $2 AND month = $3
	`

	_, err := r.db.conn.ExecContext(ctx, query, apiKeyID, year, month)
	if err != nil {
		return fmt.Errorf("failed to delete monthly usage summary: %w", err)
	}

	return nil
}