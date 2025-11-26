package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{
			name:     "simple password",
			password: "password123",
		},
		{
			name:     "complex password",
			password: "P@ssw0rd!@#$%^&*()",
		},
		{
			name:     "empty password",
			password: "",
		},
		{
			name:     "long password",
			password: "this-is-a-very-long-password-with-many-characters-1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashPassword(tt.password)

			// SHA256 produces 64 hex characters
			if len(hash) != 64 {
				t.Errorf("HashPassword() length = %d, want 64", len(hash))
			}

			// Hash should be consistent
			hash2 := HashPassword(tt.password)
			if hash != hash2 {
				t.Errorf("HashPassword() not consistent: first=%s, second=%s", hash, hash2)
			}

			// Different passwords should produce different hashes
			differentHash := HashPassword(tt.password + "x")
			if hash == differentHash && tt.password != "" {
				t.Errorf("HashPassword() produced same hash for different passwords")
			}
		})
	}
}

func TestHashPasswordDifferentInputs(t *testing.T) {
	password1 := "password1"
	password2 := "password2"

	hash1 := HashPassword(password1)
	hash2 := HashPassword(password2)

	if hash1 == hash2 {
		t.Error("HashPassword() produced same hash for different passwords")
	}

	// Verify both are valid SHA256 hashes (64 hex characters)
	if len(hash1) != 64 || len(hash2) != 64 {
		t.Errorf("HashPassword() produced invalid hash lengths: hash1=%d, hash2=%d", len(hash1), len(hash2))
	}
}
