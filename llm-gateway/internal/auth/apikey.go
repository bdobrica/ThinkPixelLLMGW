package auth

import (
	"context"
	"crypto/sha256"
)

// APIKeyRecord is the minimal view of an API key needed at request time.
type APIKeyRecord struct {
	ID   string
	Name string
	// TODO: add allowed models, tags, limits, etc., or fetch from DB directly.
}

// APIKeyStore resolves plaintext API keys into stored records.
type APIKeyStore interface {
	Lookup(ctx context.Context, plaintextKey string) (*APIKeyRecord, error)
}

// InMemoryAPIKeyStore is a placeholder store useful for early local testing.
type InMemoryAPIKeyStore struct {
	// TODO: add a map/hash for demo keys.
}

func NewInMemoryAPIKeyStore() *InMemoryAPIKeyStore {
	return &InMemoryAPIKeyStore{}
}

func (s *InMemoryAPIKeyStore) Lookup(ctx context.Context, plaintextKey string) (*APIKeyRecord, error) {
	// TODO: replace with real lookup (DB + hash check)
	return &APIKeyRecord{
		ID:   HashKey(plaintextKey),
		Name: "dummy-key",
	}, nil
}

// HashKey hashes the plaintext key; later this should match DB-stored hashes.
func HashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return string(sum[:])
}
