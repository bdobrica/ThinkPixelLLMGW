package httpapi

import (
	"context"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"

	"github.com/google/uuid"
)

// AdminStoreAdapter adapts storage repositories to auth.AdminStore interface
type AdminStoreAdapter struct {
	userRepo  *storage.AdminUserRepository
	tokenRepo *storage.AdminTokenRepository
}

// NewAdminStoreAdapter creates a new admin store adapter
func NewAdminStoreAdapter(userRepo *storage.AdminUserRepository, tokenRepo *storage.AdminTokenRepository) *AdminStoreAdapter {
	return &AdminStoreAdapter{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
	}
}

// GetAdminUserByEmail retrieves an admin user by email
func (a *AdminStoreAdapter) GetAdminUserByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	return a.userRepo.GetByEmail(ctx, email)
}

// GetAdminTokenByServiceName retrieves an admin token by service name
func (a *AdminStoreAdapter) GetAdminTokenByServiceName(ctx context.Context, serviceName string) (*models.AdminToken, error) {
	return a.tokenRepo.GetByServiceName(ctx, serviceName)
}

// UpdateAdminUserLastLogin updates the last login timestamp for a user
func (a *AdminStoreAdapter) UpdateAdminUserLastLogin(ctx context.Context, id uuid.UUID) error {
	return a.userRepo.UpdateLastLogin(ctx, id)
}

// UpdateAdminTokenLastUsed updates the last used timestamp for a token
func (a *AdminStoreAdapter) UpdateAdminTokenLastUsed(ctx context.Context, id uuid.UUID) error {
	return a.tokenRepo.UpdateLastUsed(ctx, id)
}
