package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
)

// ProviderRepository handles provider database operations
type ProviderRepository struct {
	db *DB
}

// NewProviderRepository creates a new provider repository
func NewProviderRepository(db *DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

// GetByName retrieves a provider by name
func (r *ProviderRepository) GetByName(ctx context.Context, name string) (*models.Provider, error) {
	var provider models.Provider
	query := `
		SELECT id, name, display_name, provider_type, encrypted_credentials,
		       config, enabled, created_at, updated_at
		FROM providers
		WHERE name = $1
	`

	err := r.db.conn.GetContext(ctx, &provider, query, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return &provider, nil
}

// GetByID retrieves a provider by ID
func (r *ProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	var provider models.Provider
	query := `
		SELECT id, name, display_name, provider_type, encrypted_credentials,
		       config, enabled, created_at, updated_at
		FROM providers
		WHERE id = $1
	`

	err := r.db.conn.GetContext(ctx, &provider, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProviderNotFound
		}
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return &provider, nil
}

// List returns all providers
func (r *ProviderRepository) List(ctx context.Context) ([]*models.Provider, error) {
	query := `
		SELECT id, name, display_name, provider_type, encrypted_credentials,
		       config, enabled, created_at, updated_at
		FROM providers
		ORDER BY name
	`

	var providers []*models.Provider
	err := r.db.conn.SelectContext(ctx, &providers, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	return providers, nil
}

// ProviderListFilters contains filter parameters for listing providers
type ProviderListFilters struct {
	Search      string
	EnabledOnly *bool
	Page        int
	PageSize    int
}

// ProviderListResult contains paginated provider list results
type ProviderListResult struct {
	Providers  []*models.Provider
	TotalCount int
	Page       int
	PageSize   int
}

// ListWithFilters returns providers with filtering and pagination
func (r *ProviderRepository) ListWithFilters(ctx context.Context, filters ProviderListFilters) (*ProviderListResult, error) {
	// Build WHERE clause
	var whereClauses []string
	var args []interface{}
	argCount := 1

	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("(name ILIKE $%d OR display_name ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+filters.Search+"%")
		argCount++
	}

	if filters.EnabledOnly != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("enabled = $%d", argCount))
		args = append(args, *filters.EnabledOnly)
		argCount++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			whereClause += " AND " + whereClauses[i]
		}
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM providers %s", whereClause)
	var totalCount int
	err := r.db.conn.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}

	// Get paginated results
	offset := (filters.Page - 1) * filters.PageSize
	dataQuery := fmt.Sprintf(`
		SELECT id, name, display_name, provider_type, encrypted_credentials,
		       config, enabled, created_at, updated_at
		FROM providers
		%s
		ORDER BY name
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount, argCount+1)

	args = append(args, filters.PageSize, offset)

	var providers []*models.Provider
	err = r.db.conn.SelectContext(ctx, &providers, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	return &ProviderListResult{
		Providers:  providers,
		TotalCount: totalCount,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
	}, nil
}

// Create creates a new provider
func (r *ProviderRepository) Create(ctx context.Context, provider *models.Provider) error {
	query := `
		INSERT INTO providers (id, name, display_name, provider_type,
		                       encrypted_credentials, config, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	if provider.ID == uuid.Nil {
		provider.ID = uuid.New()
	}

	err := r.db.conn.QueryRowxContext(
		ctx, query,
		provider.ID, provider.Name, provider.DisplayName, provider.ProviderType,
		provider.EncryptedCredentials, provider.Config, provider.Enabled,
	).Scan(&provider.CreatedAt, &provider.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	return nil
}

// Update updates an existing provider
func (r *ProviderRepository) Update(ctx context.Context, provider *models.Provider) error {
	query := `
		UPDATE providers
		SET name = $2, display_name = $3, provider_type = $4,
		    encrypted_credentials = $5, config = $6, enabled = $7
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.conn.QueryRowxContext(
		ctx, query,
		provider.ID, provider.Name, provider.DisplayName, provider.ProviderType,
		provider.EncryptedCredentials, provider.Config, provider.Enabled,
	).Scan(&provider.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrProviderNotFound
		}
		return fmt.Errorf("failed to update provider: %w", err)
	}

	return nil
}

// Delete deletes a provider
func (r *ProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM providers WHERE id = $1"
	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrProviderNotFound
	}

	return nil
}
