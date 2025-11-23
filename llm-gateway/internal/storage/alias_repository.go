package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"gateway/internal/models"
)

// ModelAliasRepository handles model alias database operations
type ModelAliasRepository struct {
	db *DB
}

// NewModelAliasRepository creates a new model alias repository
func NewModelAliasRepository(db *DB) *ModelAliasRepository {
	return &ModelAliasRepository{db: db}
}

// GetByAlias retrieves a model alias by its alias name
func (r *ModelAliasRepository) GetByAlias(ctx context.Context, alias string) (*models.ModelAlias, error) {
	var modelAlias models.ModelAlias
	query := `
		SELECT id, alias, target_model_id, provider_id, custom_config, 
		       enabled, created_at, updated_at
		FROM model_aliases
		WHERE alias = $1 AND enabled = true
	`

	err := r.db.conn.GetContext(ctx, &modelAlias, query, alias)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrModelAliasNotFound
		}
		return nil, fmt.Errorf("failed to get model alias: %w", err)
	}

	return &modelAlias, nil
}

// GetByID retrieves a model alias by ID
func (r *ModelAliasRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ModelAlias, error) {
	var modelAlias models.ModelAlias
	query := `
		SELECT id, alias, target_model_id, provider_id, custom_config,
		       enabled, created_at, updated_at
		FROM model_aliases
		WHERE id = $1
	`

	err := r.db.conn.GetContext(ctx, &modelAlias, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrModelAliasNotFound
		}
		return nil, fmt.Errorf("failed to get model alias: %w", err)
	}

	return &modelAlias, nil
}

// List returns all model aliases
func (r *ModelAliasRepository) List(ctx context.Context) ([]*models.ModelAlias, error) {
	query := `
		SELECT id, alias, target_model_id, provider_id, custom_config,
		       enabled, created_at, updated_at
		FROM model_aliases
		ORDER BY alias
	`

	var aliases []*models.ModelAlias
	err := r.db.conn.SelectContext(ctx, &aliases, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list model aliases: %w", err)
	}

	return aliases, nil
}

// ListEnabled returns only enabled model aliases
func (r *ModelAliasRepository) ListEnabled(ctx context.Context) ([]*models.ModelAlias, error) {
	query := `
		SELECT id, alias, target_model_id, provider_id, custom_config,
		       enabled, created_at, updated_at
		FROM model_aliases
		WHERE enabled = true
		ORDER BY alias
	`

	var aliases []*models.ModelAlias
	err := r.db.conn.SelectContext(ctx, &aliases, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list enabled model aliases: %w", err)
	}

	return aliases, nil
}

// Create creates a new model alias
func (r *ModelAliasRepository) Create(ctx context.Context, alias *models.ModelAlias) error {
	query := `
		INSERT INTO model_aliases (id, alias, target_model_id, provider_id, custom_config, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`

	if alias.ID == uuid.Nil {
		alias.ID = uuid.New()
	}

	err := r.db.conn.QueryRowxContext(
		ctx, query,
		alias.ID, alias.Alias, alias.TargetModelID, alias.ProviderID,
		alias.CustomConfig, alias.Enabled,
	).Scan(&alias.CreatedAt, &alias.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create model alias: %w", err)
	}

	return nil
}

// Update updates an existing model alias
func (r *ModelAliasRepository) Update(ctx context.Context, alias *models.ModelAlias) error {
	query := `
		UPDATE model_aliases
		SET alias = $2, target_model_id = $3, provider_id = $4, 
		    custom_config = $5, enabled = $6
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.conn.QueryRowxContext(
		ctx, query,
		alias.ID, alias.Alias, alias.TargetModelID, alias.ProviderID,
		alias.CustomConfig, alias.Enabled,
	).Scan(&alias.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrModelAliasNotFound
		}
		return fmt.Errorf("failed to update model alias: %w", err)
	}

	return nil
}

// Delete deletes a model alias
func (r *ModelAliasRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM model_aliases WHERE id = $1"
	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model alias: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrModelAliasNotFound
	}

	return nil
}

// ListByProvider returns all aliases for a specific provider
func (r *ModelAliasRepository) ListByProvider(ctx context.Context, providerID uuid.UUID) ([]*models.ModelAlias, error) {
	query := `
		SELECT id, alias, target_model_id, provider_id, custom_config,
		       enabled, created_at, updated_at
		FROM model_aliases
		WHERE provider_id = $1
		ORDER BY alias
	`

	var aliases []*models.ModelAlias
	err := r.db.conn.SelectContext(ctx, &aliases, query, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list aliases by provider: %w", err)
	}

	return aliases, nil
}

// ListByModel returns all aliases for a specific model
func (r *ModelAliasRepository) ListByModel(ctx context.Context, modelID uuid.UUID) ([]*models.ModelAlias, error) {
	query := `
		SELECT id, alias, target_model_id, provider_id, custom_config,
		       enabled, created_at, updated_at
		FROM model_aliases
		WHERE target_model_id = $1
		ORDER BY alias
	`

	var aliases []*models.ModelAlias
	err := r.db.conn.SelectContext(ctx, &aliases, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to list aliases by model: %w", err)
	}

	return aliases, nil
}
