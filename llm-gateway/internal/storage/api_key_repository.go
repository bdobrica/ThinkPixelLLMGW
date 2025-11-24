package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"gateway/internal/models"
)

// APIKeyRepository handles API key database operations with caching
type APIKeyRepository struct {
	db    *DB
	cache *LRUCache
}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *DB) *APIKeyRepository {
	return &APIKeyRepository{
		db:    db,
		cache: db.GetAPIKeyCache(),
	}
}

// GetByHash retrieves an API key by its hash (with caching)
func (r *APIKeyRepository) GetByHash(ctx context.Context, keyHash string) (*models.APIKey, error) {
	// Check cache first
	if cached, found := r.cache.Get(keyHash); found {
		return cached.(*models.APIKey), nil
	}

	// Query database
	var key models.APIKey
	query := `
		SELECT id, name, key_hash, allowed_models, rate_limit_per_minute, 
		       monthly_budget_usd, enabled, expires_at, created_at, updated_at
		FROM api_keys
		WHERE key_hash = $1 AND enabled = true
	`

	err := r.db.conn.GetContext(ctx, &key, query, keyHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Load metadata
	if err := r.loadMetadata(ctx, &key); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	// Cache the result
	r.cache.Set(keyHash, &key)

	return &key, nil
}

// GetByID retrieves an API key by ID
func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var key models.APIKey
	query := `
		SELECT id, name, key_hash, allowed_models, rate_limit_per_minute,
		       monthly_budget_usd, enabled, expires_at, created_at, updated_at
		FROM api_keys
		WHERE id = $1
	`

	err := r.db.conn.GetContext(ctx, &key, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// Load metadata
	if err := r.loadMetadata(ctx, &key); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	return &key, nil
}

// loadMetadata loads metadata for an API key
func (r *APIKeyRepository) loadMetadata(ctx context.Context, key *models.APIKey) error {
	query := `
		SELECT metadata_type, key, value
		FROM key_metadata
		WHERE api_key_id = $1
	`

	rows, err := r.db.conn.QueryxContext(ctx, query, key.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	key.Metadata = make(map[string]map[string]string)

	for rows.Next() {
		var metadataType, k, value string
		if err := rows.Scan(&metadataType, &k, &value); err != nil {
			return err
		}

		if key.Metadata[metadataType] == nil {
			key.Metadata[metadataType] = make(map[string]string)
		}
		key.Metadata[metadataType][k] = value
	}

	return rows.Err()
}

// Create creates a new API key
func (r *APIKeyRepository) Create(ctx context.Context, key *models.APIKey) error {
	query := `
		INSERT INTO api_keys (id, name, key_hash, allowed_models, rate_limit_per_minute,
		                      monthly_budget_usd, enabled, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}

	err := r.db.conn.QueryRowxContext(
		ctx, query,
		key.ID, key.Name, key.KeyHash, key.AllowedModels, key.RateLimitPerMinute,
		key.MonthlyBudgetUSD, key.Enabled, key.ExpiresAt,
	).Scan(&key.CreatedAt, &key.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	// Invalidate cache
	r.cache.Delete(key.KeyHash)

	return nil
}

// Update updates an existing API key
func (r *APIKeyRepository) Update(ctx context.Context, key *models.APIKey) error {
	query := `
		UPDATE api_keys
		SET name = $2, allowed_models = $3, rate_limit_per_minute = $4,
		    monthly_budget_usd = $5, enabled = $6, expires_at = $7
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.conn.QueryRowxContext(
		ctx, query,
		key.ID, key.Name, key.AllowedModels, key.RateLimitPerMinute,
		key.MonthlyBudgetUSD, key.Enabled, key.ExpiresAt,
	).Scan(&key.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ErrAPIKeyNotFound
		}
		return fmt.Errorf("failed to update API key: %w", err)
	}

	// Invalidate cache
	r.cache.Delete(key.KeyHash)

	return nil
}

// Delete deletes an API key
func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Get key hash before deletion to invalidate cache
	var keyHash string
	err := r.db.conn.GetContext(ctx, &keyHash, "SELECT key_hash FROM api_keys WHERE id = $1", id)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get key hash: %w", err)
	}

	query := "DELETE FROM api_keys WHERE id = $1"
	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAPIKeyNotFound
	}

	// Invalidate cache
	if keyHash != "" {
		r.cache.Delete(keyHash)
	}

	return nil
}

// List returns all API keys (paginated)
func (r *APIKeyRepository) List(ctx context.Context, limit, offset int) ([]*models.APIKey, error) {
	query := `
		SELECT id, name, key_hash, allowed_models, rate_limit_per_minute,
		       monthly_budget_usd, enabled, expires_at, created_at, updated_at
		FROM api_keys
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	var keys []*models.APIKey
	err := r.db.conn.SelectContext(ctx, &keys, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	// Load metadata for each key
	for _, key := range keys {
		if err := r.loadMetadata(ctx, key); err != nil {
			return nil, fmt.Errorf("failed to load metadata: %w", err)
		}
	}

	return keys, nil
}

// SetMetadata sets metadata for an API key
func (r *APIKeyRepository) SetMetadata(ctx context.Context, apiKeyID uuid.UUID, metadataType, key, value string) error {
	query := `
		INSERT INTO key_metadata (api_key_id, metadata_type, key, value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (api_key_id, metadata_type, key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`

	_, err := r.db.conn.ExecContext(ctx, query, apiKeyID, metadataType, key, value)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	// Invalidate cache (get key hash first)
	var keyHash string
	if err := r.db.conn.GetContext(ctx, &keyHash, "SELECT key_hash FROM api_keys WHERE id = $1", apiKeyID); err == nil {
		r.cache.Delete(keyHash)
	}

	return nil
}

// DeleteMetadata deletes metadata for an API key
func (r *APIKeyRepository) DeleteMetadata(ctx context.Context, apiKeyID uuid.UUID, metadataType, key string) error {
	query := "DELETE FROM key_metadata WHERE api_key_id = $1 AND metadata_type = $2 AND key = $3"

	_, err := r.db.conn.ExecContext(ctx, query, apiKeyID, metadataType, key)
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	// Invalidate cache
	var keyHash string
	if err := r.db.conn.GetContext(ctx, &keyHash, "SELECT key_hash FROM api_keys WHERE id = $1", apiKeyID); err == nil {
		r.cache.Delete(keyHash)
	}

	return nil
}

// InvalidateCache removes an API key from the cache
func (r *APIKeyRepository) InvalidateCache(keyHash string) {
	r.cache.Delete(keyHash)
}
