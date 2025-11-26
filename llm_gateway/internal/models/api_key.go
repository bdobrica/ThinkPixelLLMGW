package models

import (
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// APIKey represents a client API key managed by the admin API.
type APIKey struct {
	ID                 uuid.UUID      `db:"id"`
	Name               string         `db:"name"`
	KeyHash            string         `db:"key_hash"` // SHA-256 hash
	AllowedModels      pq.StringArray `db:"allowed_models"`
	RateLimitPerMinute int            `db:"rate_limit_per_minute"`
	MonthlyBudgetUSD   *float64       `db:"monthly_budget_usd"` // NULL = unlimited
	Enabled            bool           `db:"enabled"`
	ExpiresAt          *time.Time     `db:"expires_at"`
	CreatedAt          time.Time      `db:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"`

	// Not stored in DB, populated from api_key_tags table
	Tags map[string]string `db:"-"` // -> key -> value
}

// AllowsModel checks if the key is allowed to call the given model (or alias).
func (k *APIKey) AllowsModel(model string) bool {
	// Empty allowed models = allow all
	if len(k.AllowedModels) == 0 {
		return true
	}
	return slices.Contains(k.AllowedModels, model)
}

// IsExpired checks if the key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid checks if the key is valid (enabled and not expired)
func (k *APIKey) IsValid() bool {
	return k.Enabled && !k.IsExpired()
}
