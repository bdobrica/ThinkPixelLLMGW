package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestAdminToken_HasRole(t *testing.T) {
	token := &AdminToken{
		ID:          uuid.New(),
		ServiceName: "ci-pipeline",
		TokenHash:   "hashed_token",
		Roles:       pq.StringArray{"editor", "viewer"},
		Enabled:     true,
	}

	assert.True(t, token.HasRole("editor"))
	assert.True(t, token.HasRole("viewer"))
	assert.False(t, token.HasRole("admin"))
	assert.False(t, token.HasRole("nonexistent"))
}

func TestAdminToken_HasAnyRole(t *testing.T) {
	token := &AdminToken{
		ID:          uuid.New(),
		ServiceName: "ci-pipeline",
		TokenHash:   "hashed_token",
		Roles:       pq.StringArray{"editor", "viewer"},
		Enabled:     true,
	}

	assert.True(t, token.HasAnyRole("editor"))
	assert.True(t, token.HasAnyRole("viewer"))
	assert.True(t, token.HasAnyRole("editor", "admin"))
	assert.True(t, token.HasAnyRole("admin", "viewer"))
	assert.False(t, token.HasAnyRole("admin"))
	assert.False(t, token.HasAnyRole("admin", "superadmin"))
}

func TestAdminToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "no expiration",
			expiresAt: nil,
			want:      false,
		},
		{
			name: "expired",
			expiresAt: func() *time.Time {
				t := time.Now().Add(-24 * time.Hour)
				return &t
			}(),
			want: true,
		},
		{
			name: "not expired yet",
			expiresAt: func() *time.Time {
				t := time.Now().Add(24 * time.Hour)
				return &t
			}(),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &AdminToken{
				ID:          uuid.New(),
				ServiceName: "test-service",
				TokenHash:   "hashed_token",
				Roles:       pq.StringArray{"viewer"},
				Enabled:     true,
				ExpiresAt:   tt.expiresAt,
			}
			assert.Equal(t, tt.want, token.IsExpired())
		})
	}
}

func TestAdminToken_IsValid(t *testing.T) {
	now := time.Now()
	futureTime := now.Add(24 * time.Hour)
	pastTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		enabled   bool
		expiresAt *time.Time
		want      bool
	}{
		{
			name:      "enabled and not expired",
			enabled:   true,
			expiresAt: nil,
			want:      true,
		},
		{
			name:      "enabled with future expiration",
			enabled:   true,
			expiresAt: &futureTime,
			want:      true,
		},
		{
			name:      "disabled",
			enabled:   false,
			expiresAt: nil,
			want:      false,
		},
		{
			name:      "expired",
			enabled:   true,
			expiresAt: &pastTime,
			want:      false,
		},
		{
			name:      "disabled and expired",
			enabled:   false,
			expiresAt: &pastTime,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &AdminToken{
				ID:          uuid.New(),
				ServiceName: "test-service",
				TokenHash:   "hashed_token",
				Roles:       pq.StringArray{"viewer"},
				Enabled:     tt.enabled,
				ExpiresAt:   tt.expiresAt,
			}
			assert.Equal(t, tt.want, token.IsValid())
		})
	}
}

func TestAdminToken_LastUsedTracking(t *testing.T) {
	token := &AdminToken{
		ID:          uuid.New(),
		ServiceName: "test-service",
		TokenHash:   "hashed_token",
		Roles:       pq.StringArray{"viewer"},
		Enabled:     true,
	}

	// Initially nil
	assert.Nil(t, token.LastUsedAt)

	// After use
	now := time.Now()
	token.LastUsedAt = &now
	assert.NotNil(t, token.LastUsedAt)
	assert.Equal(t, now.Unix(), token.LastUsedAt.Unix())
}
