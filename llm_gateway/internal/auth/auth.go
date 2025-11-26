package auth

import (
	"context"
	"net/http"

	"llm_gateway/internal/config"
	"llm_gateway/internal/utils"
)

// AuthHandler exchanges API key for a JWT
// Requires APIKeyStore to be injected
func AuthHandler(store APIKeyStore, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			utils.RespondWithError(w, http.StatusBadRequest, "API Key is required")
			return
		}

		// Validate API key using the store
		ctx := context.Background()
		keyRecord, err := store.Lookup(ctx, apiKey)
		if err != nil {
			if err == ErrKeyNotFound {
				utils.RespondWithError(w, http.StatusUnauthorized, "Invalid API Key")
				return
			}
			utils.RespondWithError(w, http.StatusInternalServerError, "Error validating API Key: "+err.Error())
			return
		}

		if keyRecord.Revoked {
			utils.RespondWithError(w, http.StatusUnauthorized, "API Key has been revoked")
			return
		}

		// Generate JWT with hashed key
		hashedKey := utils.HashString(apiKey)
		jwt, exp, err := GenerateJWT(apiKey, hashedKey, cfg)
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error generating token: "+err.Error())
			return
		}

		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
			"token": jwt,
			"exp":   exp,
		})
	}
}
