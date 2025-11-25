package middleware

import (
	"context"
	"net/http"
	"strings"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/utils"
)

// ContextKey defines the type for context keys to avoid conflicts
type ContextKey string

const (
	// APIKeyRecordKey is the context key for storing the authenticated API key record
	APIKeyRecordKey ContextKey = "apiKeyRecord"
)

// APIKeyMiddleware validates API keys for protected routes and adds the key record to the request context
func APIKeyMiddleware(store auth.APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from header
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				// Try Authorization header with "Bearer" prefix
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
					apiKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if apiKey == "" {
				utils.RespondWithError(w, http.StatusUnauthorized, "Missing API key")
				return
			}

			// Validate the API key using the store
			ctx := r.Context()
			keyRecord, err := store.Lookup(ctx, apiKey)
			if err != nil {
				if err == auth.ErrKeyNotFound {
					utils.RespondWithError(w, http.StatusUnauthorized, "Invalid API key")
					return
				}
				utils.RespondWithError(w, http.StatusInternalServerError, "Error validating API key: "+err.Error())
				return
			}

			// Check if key is revoked
			if keyRecord.Revoked {
				utils.RespondWithError(w, http.StatusUnauthorized, "API key has been revoked")
				return
			}

			// Add the key record to the request context
			ctx = context.WithValue(r.Context(), APIKeyRecordKey, keyRecord)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAPIKeyRecord retrieves the API key record from the request context
func GetAPIKeyRecord(ctx context.Context) (*auth.APIKeyRecord, bool) {
	record, ok := ctx.Value(APIKeyRecordKey).(*auth.APIKeyRecord)
	return record, ok
}
