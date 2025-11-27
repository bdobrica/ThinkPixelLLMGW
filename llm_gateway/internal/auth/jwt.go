package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"llm_gateway/internal/config"
	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
	"llm_gateway/internal/utils"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// AdminAuthType represents the type of admin authentication
type AdminAuthType string

const (
	AdminAuthTypeUser  AdminAuthType = "user"
	AdminAuthTypeToken AdminAuthType = "token"
)

// AdminClaims represents JWT claims for admin authentication
type AdminClaims struct {
	AdminID     string        `json:"admin_id"`
	AuthType    AdminAuthType `json:"auth_type"`
	Roles       []string      `json:"roles"`
	Email       string        `json:"email,omitempty"`        // Only for user auth
	ServiceName string        `json:"service_name,omitempty"` // Only for token auth
	jwt.RegisteredClaims
}

// AdminStore defines the interface for admin authentication
type AdminStore interface {
	GetAdminUserByEmail(ctx context.Context, email string) (*models.AdminUser, error)
	GetAdminTokenByServiceName(ctx context.Context, serviceName string) (*models.AdminToken, error)
	UpdateAdminUserLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateAdminTokenLastUsed(ctx context.Context, id uuid.UUID) error
}

// GenerateAdminJWTWithPassword authenticates admin user with email/password and generates JWT
func GenerateAdminJWTWithPassword(ctx context.Context, email, password string, store AdminStore, cfg *config.Config) (string, int64, error) {
	// Get admin user by email
	user, err := store.GetAdminUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrAdminUserNotFound) {
			return "", 0, errors.New("invalid credentials")
		}
		return "", 0, fmt.Errorf("failed to get admin user: %w", err)
	}

	// Check if user is enabled
	if !user.IsValid() {
		return "", 0, errors.New("account disabled")
	}

	// Verify password using Argon2
	valid, err := utils.VerifyPasswordArgon2(password, user.PasswordHash)
	if err != nil {
		return "", 0, fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return "", 0, errors.New("invalid credentials")
	}

	// Update last login
	if err := store.UpdateAdminUserLastLogin(ctx, user.ID); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to update last login for user %s: %v\n", user.Email, err)
	}

	// Generate JWT
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &AdminClaims{
		AdminID:  user.ID.String(),
		AuthType: AdminAuthTypeUser,
		Roles:    user.Roles,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.Email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", 0, fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, expirationTime.Unix(), nil
}

// GenerateAdminJWTWithToken authenticates admin token and generates JWT
func GenerateAdminJWTWithToken(ctx context.Context, serviceName, token string, store AdminStore, cfg *config.Config) (string, int64, error) {
	// Get admin token by service name
	adminToken, err := store.GetAdminTokenByServiceName(ctx, serviceName)
	if err != nil {
		if errors.Is(err, storage.ErrAdminTokenNotFound) {
			return "", 0, errors.New("invalid credentials")
		}
		return "", 0, fmt.Errorf("failed to get admin token: %w", err)
	}

	// Verify token using Argon2
	valid, err := utils.VerifyPasswordArgon2(token, adminToken.TokenHash)
	if err != nil {
		return "", 0, fmt.Errorf("failed to verify token: %w", err)
	}
	if !valid {
		return "", 0, errors.New("invalid token")
	}

	// Check if token is valid (enabled and not expired)
	if !adminToken.IsValid() {
		return "", 0, errors.New("token disabled or expired")
	}

	// Update last used
	if err := store.UpdateAdminTokenLastUsed(ctx, adminToken.ID); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to update last used for token %s: %v\n", adminToken.ServiceName, err)
	}

	// Generate JWT
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &AdminClaims{
		AdminID:     adminToken.ID.String(),
		AuthType:    AdminAuthTypeToken,
		Roles:       adminToken.Roles,
		ServiceName: adminToken.ServiceName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   adminToken.ServiceName,
		},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := jwtToken.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", 0, fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, expirationTime.Unix(), nil
}

// ValidateAdminJWT verifies and parses an admin JWT
func ValidateAdminJWT(tokenString string, cfg *config.Config) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(token *jwt.Token) (interface{}, error) {
		return cfg.JWTSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
