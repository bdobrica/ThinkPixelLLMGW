package httpapi

import (
	"context"
	"fmt"

	"gateway/internal/auth"
	"gateway/internal/models"
	"gateway/internal/storage"
)

// DatabaseAPIKeyStore implements APIKeyStore using the database repository
type DatabaseAPIKeyStore struct {
	repo *storage.APIKeyRepository
}

// NewDatabaseAPIKeyStore creates a new database-backed API key store
func NewDatabaseAPIKeyStore(repo *storage.APIKeyRepository) *DatabaseAPIKeyStore {
	return &DatabaseAPIKeyStore{
		repo: repo,
	}
}

// Lookup finds an API key by its plaintext value
func (s *DatabaseAPIKeyStore) Lookup(ctx context.Context, plaintextKey string) (*models.APIKey, error) {
	// Hash the plaintext key
	hashedKey := auth.HashKey(plaintextKey)
	
	// Look up in database (with caching)
	apiKey, err := s.repo.GetByHash(ctx, hashedKey)
	if err != nil {
		if err == storage.ErrAPIKeyNotFound {
			return nil, storage.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("failed to lookup API key: %w", err)
	}
	
	return apiKey, nil
}

