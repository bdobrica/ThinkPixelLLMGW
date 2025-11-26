package auth

import (
	"context"
	"slices"

	"llm_gateway/internal/utils"
)

// APIKeyRecord is the view of an API key needed at request time.
type APIKeyRecord struct {
	ID            string
	Name          string
	AllowedModels []string
	Tags          map[string]string
	Revoked       bool
}

// AllowsModel checks whether this key may call a given model/alias.
func (k *APIKeyRecord) AllowsModel(model string) bool {
	// If no models configured, allow everything (for early testing).
	if len(k.AllowedModels) == 0 {
		return true
	}
	return slices.Contains(k.AllowedModels, model)
}

// APIKeyStore resolves plaintext API keys into stored records.
type APIKeyStore interface {
	Lookup(ctx context.Context, plaintextKey string) (*APIKeyRecord, error)
}

// InMemoryAPIKeyStore is a placeholder store useful for early local testing.
type InMemoryAPIKeyStore struct {
	// map of hash(API key) -> record
	keys map[string]*APIKeyRecord
}

func NewInMemoryAPIKeyStore() *InMemoryAPIKeyStore {
	s := &InMemoryAPIKeyStore{
		keys: make(map[string]*APIKeyRecord),
	}

	// Seed with a demo key: "demo-key"
	hash := utils.HashString("demo-key")
	s.keys[hash] = &APIKeyRecord{
		ID:            "demo-key-id",
		Name:          "Demo Key",
		AllowedModels: []string{}, // all models
		Tags:          map[string]string{"env": "dev"},
		Revoked:       false,
	}

	return s
}

func (s *InMemoryAPIKeyStore) Lookup(ctx context.Context, plaintextKey string) (*APIKeyRecord, error) {
	hash := utils.HashString(plaintextKey)
	rec, ok := s.keys[hash]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return rec, nil
}
