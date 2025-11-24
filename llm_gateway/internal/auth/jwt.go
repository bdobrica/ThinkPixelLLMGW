package auth

import "context"

// JWTClaims represents the authenticated principal.
type JWTClaims struct {
	Subject string
	// TODO: add roles, permissions, etc.
}

// JWTManager verifies admin JWT tokens.
type JWTManager interface {
	Verify(ctx context.Context, token string) (*JWTClaims, error)
}

// NoopJWTManager accepts any token and returns dummy claims.
// Replace with real JWT verification.
type NoopJWTManager struct{}

func NewNoopJWTManager() *NoopJWTManager {
	return &NoopJWTManager{}
}

func (m *NoopJWTManager) Verify(ctx context.Context, token string) (*JWTClaims, error) {
	// TODO: verify JWT signature, expiry, etc.
	return &JWTClaims{Subject: "admin"}, nil
}
