package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestAdminUser_HasRole(t *testing.T) {
	user := &AdminUser{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: "hashed_password",
		Roles:        pq.StringArray{"admin", "viewer"},
		Enabled:      true,
	}

	assert.True(t, user.HasRole("admin"))
	assert.True(t, user.HasRole("viewer"))
	assert.False(t, user.HasRole("editor"))
	assert.False(t, user.HasRole("nonexistent"))
}

func TestAdminUser_HasAnyRole(t *testing.T) {
	user := &AdminUser{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: "hashed_password",
		Roles:        pq.StringArray{"admin", "viewer"},
		Enabled:      true,
	}

	assert.True(t, user.HasAnyRole("admin"))
	assert.True(t, user.HasAnyRole("viewer"))
	assert.True(t, user.HasAnyRole("admin", "editor"))
	assert.True(t, user.HasAnyRole("editor", "viewer"))
	assert.False(t, user.HasAnyRole("editor"))
	assert.False(t, user.HasAnyRole("editor", "superadmin"))
}

func TestAdminUser_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled user",
			enabled: true,
			want:    true,
		},
		{
			name:    "disabled user",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &AdminUser{
				ID:           uuid.New(),
				Email:        "user@example.com",
				PasswordHash: "hashed_password",
				Roles:        pq.StringArray{"viewer"},
				Enabled:      tt.enabled,
			}
			assert.Equal(t, tt.want, user.IsValid())
		})
	}
}

func TestAdminUser_LastLoginTracking(t *testing.T) {
	user := &AdminUser{
		ID:           uuid.New(),
		Email:        "user@example.com",
		PasswordHash: "hashed_password",
		Roles:        pq.StringArray{"viewer"},
		Enabled:      true,
	}

	// Initially nil
	assert.Nil(t, user.LastLoginAt)

	// After login
	now := time.Now()
	user.LastLoginAt = &now
	assert.NotNil(t, user.LastLoginAt)
	assert.Equal(t, now.Unix(), user.LastLoginAt.Unix())
}
