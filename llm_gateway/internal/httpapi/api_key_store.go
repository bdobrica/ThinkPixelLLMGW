package httpapi

import (
	"context"
	"fmt"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
)

// DatabaseAPIKeyStore implements auth.APIKeyStore using the database repository
type DatabaseAPIKeyStore struct {
	repo *storage.APIKeyRepository
}

// NewDatabaseAPIKeyStore creates a new database-backed API key store
func NewDatabaseAPIKeyStore(repo *storage.APIKeyRepository) *DatabaseAPIKeyStore {
	return &DatabaseAPIKeyStore{
		repo: repo,
	}
}

// Lookup finds an API key by its plaintext value and returns an auth.APIKeyRecord
func (s *DatabaseAPIKeyStore) Lookup(ctx context.Context, plaintextKey string) (*auth.APIKeyRecord, error) {
	// Hash the plaintext key
	hashedKey := utils.HashPassword(plaintextKey)

	// Look up in database (with caching)
	apiKey, err := s.repo.GetByHash(ctx, hashedKey)
	if err != nil {
		if err == storage.ErrAPIKeyNotFound {
			return nil, auth.ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to lookup API key: %w", err)
	}

	// Convert models.APIKey to auth.APIKeyRecord
	record := &auth.APIKeyRecord{
		ID:                 apiKey.ID.String(),
		Name:               apiKey.Name,
		AllowedModels:      apiKey.AllowedModels,
		RateLimitPerMinute: apiKey.RateLimitPerMinute,
		Tags:               apiKey.Tags,
		Revoked:            !apiKey.Enabled || apiKey.IsExpired(), // Revoked if disabled or expired
	}

	return record, nil
}
