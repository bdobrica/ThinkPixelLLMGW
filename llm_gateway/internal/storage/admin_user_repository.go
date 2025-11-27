package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
)

// AdminUserRepository handles admin user database operations
type AdminUserRepository struct {
	db *DB
}

// NewAdminUserRepository creates a new admin user repository
func NewAdminUserRepository(db *DB) *AdminUserRepository {
	return &AdminUserRepository{
		db: db,
	}
}

// GetByEmail retrieves an admin user by email
func (r *AdminUserRepository) GetByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	var user models.AdminUser
	query := `
		SELECT id, email, password_hash, roles, enabled, last_login_at, created_at, updated_at
		FROM admin_users
		WHERE email = $1
	`

	err := r.db.conn.GetContext(ctx, &user, query, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAdminUserNotFound
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	return &user, nil
}

// GetByID retrieves an admin user by ID
func (r *AdminUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error) {
	var user models.AdminUser
	query := `
		SELECT id, email, password_hash, roles, enabled, last_login_at, created_at, updated_at
		FROM admin_users
		WHERE id = $1
	`

	err := r.db.conn.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAdminUserNotFound
		}
		return nil, fmt.Errorf("failed to get admin user: %w", err)
	}

	return &user, nil
}

// Create creates a new admin user
func (r *AdminUserRepository) Create(ctx context.Context, user *models.AdminUser) error {
	query := `
		INSERT INTO admin_users (id, email, password_hash, roles, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	err := r.db.conn.QueryRowContext(
		ctx, query,
		user.ID, user.Email, user.PasswordHash, user.Roles, user.Enabled,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	return nil
}

// Update updates an existing admin user
func (r *AdminUserRepository) Update(ctx context.Context, user *models.AdminUser) error {
	query := `
		UPDATE admin_users
		SET email = $2, password_hash = $3, roles = $4, enabled = $5, last_login_at = $6
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.conn.QueryRowContext(
		ctx, query,
		user.ID, user.Email, user.PasswordHash, user.Roles, user.Enabled, user.LastLoginAt,
	).Scan(&user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrAdminUserNotFound
		}
		return fmt.Errorf("failed to update admin user: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp for a user
func (r *AdminUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE admin_users
		SET last_login_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAdminUserNotFound
	}

	return nil
}

// Delete deletes an admin user by ID
func (r *AdminUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM admin_users WHERE id = $1`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete admin user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAdminUserNotFound
	}

	return nil
}

// List retrieves all admin users with optional filters
func (r *AdminUserRepository) List(ctx context.Context, enabledOnly bool) ([]*models.AdminUser, error) {
	query := `
		SELECT id, email, password_hash, roles, enabled, last_login_at, created_at, updated_at
		FROM admin_users
	`

	if enabledOnly {
		query += " WHERE enabled = true"
	}

	query += " ORDER BY created_at DESC"

	var users []*models.AdminUser
	err := r.db.conn.SelectContext(ctx, &users, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list admin users: %w", err)
	}

	return users, nil
}
