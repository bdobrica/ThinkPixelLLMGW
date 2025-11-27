package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// AdminToken represents a service account for management API access
// Authentication is token-based with Argon2 token hashing
type AdminToken struct {
	ID          uuid.UUID      `db:"id"`
	ServiceName string         `db:"service_name"`
	TokenHash   string         `db:"token_hash"` // Argon2 hash
	Roles       pq.StringArray `db:"roles"`      // e.g., ["admin", "viewer", "editor"]
	Enabled     bool           `db:"enabled"`
	ExpiresAt   *time.Time     `db:"expires_at"`
	LastUsedAt  *time.Time     `db:"last_used_at"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

// HasRole checks if the token has a specific role
func (t *AdminToken) HasRole(role string) bool {
	for _, r := range t.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the token has any of the specified roles
func (t *AdminToken) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if t.HasRole(role) {
			return true
		}
	}
	return false
}

// IsExpired checks if the token has expired
func (t *AdminToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// IsValid checks if the token is enabled and not expired
func (t *AdminToken) IsValid() bool {
	return t.Enabled && !t.IsExpired()
}
