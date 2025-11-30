package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
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

	// Load tags
	if err := r.loadTags(ctx, &modelAlias); err != nil {
		return nil, fmt.Errorf("failed to load tags: %w", err)
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

	// Load tags
	if err := r.loadTags(ctx, &modelAlias); err != nil {
		return nil, fmt.Errorf("failed to load tags: %w", err)
	}

	return &modelAlias, nil
}

// loadTags loads tags for a model alias
func (r *ModelAliasRepository) loadTags(ctx context.Context, alias *models.ModelAlias) error {
	query := `
		SELECT key, value
		FROM model_alias_tags
		WHERE model_alias_id = $1
	`

	rows, err := r.db.conn.QueryxContext(ctx, query, alias.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	alias.Tags = make(map[string]string)

	for rows.Next() {
		var k, value string
		if err := rows.Scan(&k, &value); err != nil {
			return err
		}
		alias.Tags[k] = value
	}

	return rows.Err()
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

	// Load tags for each alias
	for _, alias := range aliases {
		if err := r.loadTags(ctx, alias); err != nil {
			return nil, fmt.Errorf("failed to load tags: %w", err)
		}
	}

	return aliases, nil
}

// AliasListFilters contains filter parameters for listing aliases
type AliasListFilters struct {
	ProviderID string
	Search     string
	Tags       map[string]string // key-value pairs to filter by
	Page       int
	PageSize   int
}

// AliasListResult contains paginated alias list results
type AliasListResult struct {
	Aliases    []*models.ModelAlias
	TotalCount int
	Page       int
	PageSize   int
}

// ListWithFilters returns aliases with filtering and pagination
func (r *ModelAliasRepository) ListWithFilters(ctx context.Context, filters AliasListFilters) (*AliasListResult, error) {
	// Build WHERE clause
	var whereClauses []string
	var args []interface{}
	argCount := 1

	if filters.ProviderID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("provider_id = $%d", argCount))
		args = append(args, filters.ProviderID)
		argCount++
	}

	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("alias ILIKE $%d", argCount))
		args = append(args, "%"+filters.Search+"%")
		argCount++
	}

	// For tag filtering, we need a subquery that checks if ALL specified tags match
	if len(filters.Tags) > 0 {
		tagConditions := []string{}
		for key, value := range filters.Tags {
			tagConditions = append(tagConditions,
				fmt.Sprintf("EXISTS (SELECT 1 FROM model_alias_tags WHERE model_alias_id = model_aliases.id AND key = $%d AND value = $%d)",
					argCount, argCount+1))
			args = append(args, key, value)
			argCount += 2
		}
		whereClauses = append(whereClauses, tagConditions...)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			whereClause += " AND " + whereClauses[i]
		}
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM model_aliases %s", whereClause)
	var totalCount int
	err := r.db.conn.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count aliases: %w", err)
	}

	// Get paginated results
	offset := (filters.Page - 1) * filters.PageSize
	dataQuery := fmt.Sprintf(`
		SELECT id, alias, target_model_id, provider_id, custom_config,
		       enabled, created_at, updated_at
		FROM model_aliases
		%s
		ORDER BY alias
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount, argCount+1)

	args = append(args, filters.PageSize, offset)

	var aliases []*models.ModelAlias
	err = r.db.conn.SelectContext(ctx, &aliases, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list aliases: %w", err)
	}

	// Load tags for each alias
	for _, alias := range aliases {
		if err := r.loadTags(ctx, alias); err != nil {
			return nil, fmt.Errorf("failed to load tags: %w", err)
		}
	}

	return &AliasListResult{
		Aliases:    aliases,
		TotalCount: totalCount,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
	}, nil
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

	// Load tags for each alias
	for _, alias := range aliases {
		if err := r.loadTags(ctx, alias); err != nil {
			return nil, fmt.Errorf("failed to load tags: %w", err)
		}
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

	// Load tags for each alias
	for _, alias := range aliases {
		if err := r.loadTags(ctx, alias); err != nil {
			return nil, fmt.Errorf("failed to load tags: %w", err)
		}
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

	// Load tags for each alias
	for _, alias := range aliases {
		if err := r.loadTags(ctx, alias); err != nil {
			return nil, fmt.Errorf("failed to load tags: %w", err)
		}
	}

	return aliases, nil
}

// SetTag sets a tag for a model alias
func (r *ModelAliasRepository) SetTag(ctx context.Context, aliasID uuid.UUID, key, value string) error {
	query := `
		INSERT INTO model_alias_tags (model_alias_id, key, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (model_alias_id, key)
		DO UPDATE SET value = EXCLUDED.value
	`

	_, err := r.db.conn.ExecContext(ctx, query, aliasID, key, value)
	if err != nil {
		return fmt.Errorf("failed to set tag: %w", err)
	}

	return nil
}

// DeleteTag deletes a tag for a model alias
func (r *ModelAliasRepository) DeleteTag(ctx context.Context, aliasID uuid.UUID, key string) error {
	query := "DELETE FROM model_alias_tags WHERE model_alias_id = $1 AND key = $2"

	_, err := r.db.conn.ExecContext(ctx, query, aliasID, key)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	return nil
}
