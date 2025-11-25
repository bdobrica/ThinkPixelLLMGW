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
