package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// AdminUser represents a human account for management API access
// Authentication is email/password based with Argon2 password hashing
type AdminUser struct {
	ID           uuid.UUID      `db:"id"`
	Email        string         `db:"email"`
	PasswordHash string         `db:"password_hash"` // Argon2 hash
	Roles        pq.StringArray `db:"roles"`         // e.g., ["admin", "viewer", "editor"]
	Enabled      bool           `db:"enabled"`
	LastLoginAt  *time.Time     `db:"last_login_at"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

// HasRole checks if the user has a specific role
func (u *AdminUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (u *AdminUser) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// IsValid checks if the user account is enabled
func (u *AdminUser) IsValid() bool {
	return u.Enabled
}
