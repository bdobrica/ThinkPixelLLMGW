package auth

import (
	"context"
	"llm_gateway/internal/config"
	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// MockAdminStore for testing
type MockAdminStore struct {
	users  map[string]*models.AdminUser
	tokens map[string]*models.AdminToken
}

func NewMockAdminStore() *MockAdminStore {
	return &MockAdminStore{
		users:  make(map[string]*models.AdminUser),
		tokens: make(map[string]*models.AdminToken),
	}
}

func (m *MockAdminStore) GetAdminUserByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	if user, ok := m.users[email]; ok {
		return user, nil
	}
	return nil, storage.ErrAdminUserNotFound
}

func (m *MockAdminStore) GetAdminTokenByServiceName(ctx context.Context, serviceName string) (*models.AdminToken, error) {
	if token, ok := m.tokens[serviceName]; ok {
		return token, nil
	}
	return nil, storage.ErrAdminTokenNotFound
}

func (m *MockAdminStore) UpdateAdminUserLastLogin(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *MockAdminStore) UpdateAdminTokenLastUsed(ctx context.Context, id uuid.UUID) error {
	return nil
}

func getTestConfig() *config.Config {
	return &config.Config{
		JWTSecret: []byte("test-secret-key-for-testing"),
	}
}

func TestHashPasswordArgon2(t *testing.T) {
	password := "test-password-123"
	hash, err := utils.HashPasswordArgon2(password)
	if err != nil {
		t.Fatalf("HashPasswordArgon2() error = %v", err)
	}

	if hash == "" {
		t.Error("HashPasswordArgon2() returned empty hash")
	}

	// Hash should start with $argon2id$
	if len(hash) < 10 || hash[:10] != "$argon2id$" {
		t.Errorf("HashPasswordArgon2() hash format invalid: %s", hash)
	}
}

func TestVerifyPasswordArgon2(t *testing.T) {
	password := "test-password-123"
	hash, err := utils.HashPasswordArgon2(password)
	if err != nil {
		t.Fatalf("HashPasswordArgon2() error = %v", err)
	}

	t.Run("valid password", func(t *testing.T) {
		valid, err := utils.VerifyPasswordArgon2(password, hash)
		if err != nil {
			t.Fatalf("VerifyPasswordArgon2() error = %v", err)
		}
		if !valid {
			t.Error("VerifyPasswordArgon2() = false, want true")
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		valid, err := utils.VerifyPasswordArgon2("wrong-password", hash)
		if err != nil {
			t.Fatalf("VerifyPasswordArgon2() error = %v", err)
		}
		if valid {
			t.Error("VerifyPasswordArgon2() = true, want false")
		}
	})

	t.Run("invalid hash format", func(t *testing.T) {
		_, err := utils.VerifyPasswordArgon2(password, "invalid-hash")
		if err == nil {
			t.Error("VerifyPasswordArgon2() error = nil, want error")
		}
	})
}

func TestGenerateAdminJWTWithPassword(t *testing.T) {
	cfg := getTestConfig()
	ctx := context.Background()
	store := NewMockAdminStore()

	// Create test user with Argon2 password hash
	password := "admin-password-123"
	passwordHash, err := utils.HashPasswordArgon2(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &models.AdminUser{
		ID:           uuid.New(),
		Email:        "admin@example.com",
		PasswordHash: passwordHash,
		Roles:        pq.StringArray{"admin", "editor"},
		Enabled:      true,
	}
	store.users[user.Email] = user

	t.Run("valid credentials", func(t *testing.T) {
		token, expTime, err := GenerateAdminJWTWithPassword(ctx, user.Email, password, store, cfg)
		if err != nil {
			t.Fatalf("GenerateAdminJWTWithPassword() error = %v", err)
		}

		if token == "" {
			t.Error("GenerateAdminJWTWithPassword() returned empty token")
		}

		if expTime <= time.Now().Unix() {
			t.Error("GenerateAdminJWTWithPassword() expiration time is in the past")
		}

		// Validate the token
		claims, err := ValidateAdminJWT(token, cfg)
		if err != nil {
			t.Fatalf("ValidateAdminJWT() error = %v", err)
		}

		if claims.AuthType != AdminAuthTypeUser {
			t.Errorf("claims.AuthType = %v, want %v", claims.AuthType, AdminAuthTypeUser)
		}
		if claims.Email != user.Email {
			t.Errorf("claims.Email = %v, want %v", claims.Email, user.Email)
		}
		if len(claims.Roles) != 2 {
			t.Errorf("len(claims.Roles) = %v, want 2", len(claims.Roles))
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		_, _, err := GenerateAdminJWTWithPassword(ctx, user.Email, "wrong-password", store, cfg)
		if err == nil {
			t.Error("GenerateAdminJWTWithPassword() error = nil, want error")
		}
	})

	t.Run("disabled user", func(t *testing.T) {
		disabledUser := *user
		disabledUser.Enabled = false
		store.users[disabledUser.Email] = &disabledUser

		_, _, err := GenerateAdminJWTWithPassword(ctx, disabledUser.Email, password, store, cfg)
		if err == nil {
			t.Error("GenerateAdminJWTWithPassword() error = nil for disabled user, want error")
		}

		// Restore enabled user
		store.users[user.Email] = user
	})
}

func TestGenerateAdminJWTWithToken(t *testing.T) {
	cfg := getTestConfig()
	ctx := context.Background()
	store := NewMockAdminStore()

	// Create test token with Argon2 hash
	rawToken := "service-token-12345"
	tokenHash, err := utils.HashPasswordArgon2(rawToken)
	if err != nil {
		t.Fatalf("Failed to hash token: %v", err)
	}

	adminToken := &models.AdminToken{
		ID:          uuid.New(),
		ServiceName: "test-service",
		TokenHash:   tokenHash,
		Roles:       pq.StringArray{"admin"},
		Enabled:     true,
	}
	// Store using service_name as key for lookup
	store.tokens[adminToken.ServiceName] = adminToken

	t.Run("valid token", func(t *testing.T) {
		token, expTime, err := GenerateAdminJWTWithToken(ctx, adminToken.ServiceName, rawToken, store, cfg)
		if err != nil {
			t.Fatalf("GenerateAdminJWTWithToken() error = %v", err)
		}

		if token == "" {
			t.Error("GenerateAdminJWTWithToken() returned empty token")
		}

		if expTime <= time.Now().Unix() {
			t.Error("GenerateAdminJWTWithToken() expiration time is in the past")
		}

		// Validate the token
		claims, err := ValidateAdminJWT(token, cfg)
		if err != nil {
			t.Fatalf("ValidateAdminJWT() error = %v", err)
		}

		if claims.AuthType != AdminAuthTypeToken {
			t.Errorf("claims.AuthType = %v, want %v", claims.AuthType, AdminAuthTypeToken)
		}
		if claims.ServiceName != adminToken.ServiceName {
			t.Errorf("claims.ServiceName = %v, want %v", claims.ServiceName, adminToken.ServiceName)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, _, err := GenerateAdminJWTWithToken(ctx, adminToken.ServiceName, "wrong-token", store, cfg)
		if err == nil {
			t.Error("GenerateAdminJWTWithToken() error = nil, want error")
		}
	})

	t.Run("invalid service name", func(t *testing.T) {
		_, _, err := GenerateAdminJWTWithToken(ctx, "unknown-service", rawToken, store, cfg)
		if err == nil {
			t.Error("GenerateAdminJWTWithToken() error = nil for unknown service, want error")
		}
	})

	t.Run("disabled token", func(t *testing.T) {
		disabledToken := *adminToken
		disabledToken.Enabled = false
		store.tokens[adminToken.ServiceName] = &disabledToken

		_, _, err := GenerateAdminJWTWithToken(ctx, adminToken.ServiceName, rawToken, store, cfg)
		if err == nil {
			t.Error("GenerateAdminJWTWithToken() error = nil for disabled token, want error")
		}

		// Restore enabled token
		store.tokens[adminToken.ServiceName] = adminToken
	})

	t.Run("expired token", func(t *testing.T) {
		expiredTime := time.Now().Add(-1 * time.Hour)
		expiredToken := *adminToken
		expiredToken.ExpiresAt = &expiredTime
		store.tokens[adminToken.ServiceName] = &expiredToken

		_, _, err := GenerateAdminJWTWithToken(ctx, adminToken.ServiceName, rawToken, store, cfg)
		if err == nil {
			t.Error("GenerateAdminJWTWithToken() error = nil for expired token, want error")
		}
	})
}
