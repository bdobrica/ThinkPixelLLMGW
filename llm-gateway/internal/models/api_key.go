package models

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents a client API key managed by the admin API.
type APIKey struct {
	ID                 uuid.UUID  `db:"id"`
	Name               string     `db:"name"`
	KeyHash            string     `db:"key_hash"` // SHA-256 hash
	AllowedModels      []string   `db:"allowed_models"`
	RateLimitPerMinute int        `db:"rate_limit_per_minute"`
	MonthlyBudgetUSD   *float64   `db:"monthly_budget_usd"` // NULL = unlimited
	Enabled            bool       `db:"enabled"`
	ExpiresAt          *time.Time `db:"expires_at"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`

	// Not stored in DB, populated from key_metadata table
	Metadata map[string]map[string]string `db:"-"` // metadata_type -> key -> value
}

// AllowsModel checks if the key is allowed to call the given model (or alias).
func (k *APIKey) AllowsModel(model string) bool {
	// Empty allowed models = allow all
	if len(k.AllowedModels) == 0 {
		return true
	}

	for _, m := range k.AllowedModels {
		if m == model {
			return true
		}
	}
	return false
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
