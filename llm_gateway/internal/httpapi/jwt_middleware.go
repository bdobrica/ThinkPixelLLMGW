package httpapi

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey int

const ctxKeyJWTClaims ctxKey = iota

// withJWTAuth wraps an admin handler with JWT verification.
func (d *Dependencies) withJWTAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		claims, err := d.JWT.Verify(r.Context(), token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyJWTClaims, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
