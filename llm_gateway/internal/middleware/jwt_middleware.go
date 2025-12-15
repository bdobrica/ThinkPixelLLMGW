package middleware

import (
	"context"
	"net/http"
	"strings"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/config"
	"llm_gateway/internal/utils"
)

// Context keys for storing authentication data
const (
	AdminClaimsKey   ContextKey = "adminClaims"
	AdminAuthTypeKey ContextKey = "adminAuthType"
	AdminIDKey       ContextKey = "adminID"
	AdminRolesKey    ContextKey = "adminRoles"
)

// AdminJWTMiddleware validates admin JWT tokens and enforces role-based access
func AdminJWTMiddleware(cfg *config.Config, requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header or X-API-Key header
			tokenString := r.Header.Get("Authorization")
			if tokenString == "" {
				tokenString = r.Header.Get("X-API-Key")
			}
			if tokenString == "" {
				utils.RespondWithError(w, http.StatusUnauthorized, "Missing authentication token")
				return
			}

			// Remove "Bearer " prefix if present
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")

			// Validate and parse admin JWT
			claims, err := auth.ValidateAdminJWT(tokenString, cfg)
			if err != nil {
				utils.RespondWithError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			// Check if user/token has required roles (if specified)
			if len(requiredRoles) > 0 {
				hasPermission := false
				for _, requiredRoleStr := range requiredRoles {
					requiredRole := auth.Role(requiredRoleStr)
					for _, userRoleStr := range claims.Roles {
						userRole := auth.Role(userRoleStr)
						// Use HasPermission method which allows admin to access viewer endpoints
						if userRole.HasPermission(requiredRole) {
							hasPermission = true
							break
						}
					}
					if hasPermission {
						break
					}
				}
				if !hasPermission {
					utils.RespondWithError(w, http.StatusForbidden, "Insufficient permissions")
					return
				}
			}

			// Embed claims into request context
			ctx := context.WithValue(r.Context(), AdminClaimsKey, claims)
			ctx = context.WithValue(ctx, AdminAuthTypeKey, claims.AuthType)
			ctx = context.WithValue(ctx, AdminIDKey, claims.AdminID)
			ctx = context.WithValue(ctx, AdminRolesKey, claims.Roles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAdminClaims retrieves the admin claims from the request context
func GetAdminClaims(ctx context.Context) (*auth.AdminClaims, bool) {
	claims, ok := ctx.Value(AdminClaimsKey).(*auth.AdminClaims)
	return claims, ok
}

// GetAdminID retrieves the admin ID from the request context
func GetAdminID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(AdminIDKey).(string)
	return id, ok
}

// GetAdminRoles retrieves the admin roles from the request context
func GetAdminRoles(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(AdminRolesKey).([]string)
	return roles, ok
}

// HasRole checks if the admin has a specific role
func HasRole(ctx context.Context, role string) bool {
	roles, ok := GetAdminRoles(ctx)
	if !ok {
		return false
	}
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
