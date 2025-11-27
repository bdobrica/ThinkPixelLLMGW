package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
)

// AdminTokenRepository handles admin token database operations
type AdminTokenRepository struct {
	db *DB
}

// NewAdminTokenRepository creates a new admin token repository
func NewAdminTokenRepository(db *DB) *AdminTokenRepository {
	return &AdminTokenRepository{
		db: db,
	}
}

// GetByTokenHash retrieves an admin token by its hash
func (r *AdminTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.AdminToken, error) {
	var token models.AdminToken
	query := `
		SELECT id, service_name, token_hash, roles, enabled, expires_at, last_used_at, created_at, updated_at
		FROM admin_tokens
		WHERE token_hash = $1
	`

	err := r.db.conn.GetContext(ctx, &token, query, tokenHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAdminTokenNotFound
		}
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	return &token, nil
}

// GetByServiceName retrieves an admin token by service name
func (r *AdminTokenRepository) GetByServiceName(ctx context.Context, serviceName string) (*models.AdminToken, error) {
	var token models.AdminToken
	query := `
		SELECT id, service_name, token_hash, roles, enabled, expires_at, last_used_at, created_at, updated_at
		FROM admin_tokens
		WHERE service_name = $1
	`

	err := r.db.conn.GetContext(ctx, &token, query, serviceName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAdminTokenNotFound
		}
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	return &token, nil
}

// GetByID retrieves an admin token by ID
func (r *AdminTokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AdminToken, error) {
	var token models.AdminToken
	query := `
		SELECT id, service_name, token_hash, roles, enabled, expires_at, last_used_at, created_at, updated_at
		FROM admin_tokens
		WHERE id = $1
	`

	err := r.db.conn.GetContext(ctx, &token, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAdminTokenNotFound
		}
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	return &token, nil
}

// Create creates a new admin token
func (r *AdminTokenRepository) Create(ctx context.Context, token *models.AdminToken) error {
	query := `
		INSERT INTO admin_tokens (id, service_name, token_hash, roles, enabled, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}

	err := r.db.conn.QueryRowContext(
		ctx, query,
		token.ID, token.ServiceName, token.TokenHash, token.Roles, token.Enabled, token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create admin token: %w", err)
	}

	return nil
}

// Update updates an existing admin token
func (r *AdminTokenRepository) Update(ctx context.Context, token *models.AdminToken) error {
	query := `
		UPDATE admin_tokens
		SET service_name = $2, token_hash = $3, roles = $4, enabled = $5, expires_at = $6, last_used_at = $7
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.conn.QueryRowContext(
		ctx, query,
		token.ID, token.ServiceName, token.TokenHash, token.Roles, token.Enabled, token.ExpiresAt, token.LastUsedAt,
	).Scan(&token.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrAdminTokenNotFound
		}
		return fmt.Errorf("failed to update admin token: %w", err)
	}

	return nil
}

// UpdateLastUsed updates the last used timestamp for a token
func (r *AdminTokenRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_tokens
		SET last_used_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update last used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAdminTokenNotFound
	}

	return nil
}

// Delete deletes an admin token by ID
func (r *AdminTokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM admin_tokens WHERE id = $1`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete admin token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAdminTokenNotFound
	}

	return nil
}

// List retrieves all admin tokens with optional filters
func (r *AdminTokenRepository) List(ctx context.Context, enabledOnly bool) ([]*models.AdminToken, error) {
	query := `
		SELECT id, service_name, token_hash, roles, enabled, expires_at, last_used_at, created_at, updated_at
		FROM admin_tokens
	`

	if enabledOnly {
		query += " WHERE enabled = true AND (expires_at IS NULL OR expires_at > NOW())"
	}

	query += " ORDER BY created_at DESC"

	var tokens []*models.AdminToken
	err := r.db.conn.SelectContext(ctx, &tokens, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list admin tokens: %w", err)
	}

	return tokens, nil
}
