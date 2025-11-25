package middleware

import (
	"context"
	"net/http"
	"strings"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/utils"
)

// JWTContextKey is the context key for storing the hashed API key from JWT
const JWTContextKey ContextKey = "jwtHashedKey"

// JWTMiddleware validates JWT for protected routes and passes the hashed key to handlers
func JWTMiddleware(store auth.APIKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.Header.Get("Authorization")
			if tokenString == "" {
				utils.RespondWithError(w, http.StatusUnauthorized, "Missing token")
				return
			}

			// Remove "Bearer " prefix if present
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")

			// Validate the JWT and extract claims
			_, err := auth.ValidateJWT(tokenString)
			if err != nil {
				utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			hashedKey, err := auth.DecodeJWT(tokenString)
			if err != nil {
				utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding token: "+err.Error())
				return
			}

			// Embed hashed key into the request context
			ctx := context.WithValue(r.Context(), JWTContextKey, hashedKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetJWTHashedKey retrieves the hashed API key from the request context
func GetJWTHashedKey(ctx context.Context) (string, bool) {
	hashedKey, ok := ctx.Value(JWTContextKey).(string)
	return hashedKey, ok
}
