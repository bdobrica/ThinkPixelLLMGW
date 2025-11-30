package httpapi

import (
	"encoding/json"
	"net/http"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/config"
	"llm_gateway/internal/middleware"
	"llm_gateway/internal/utils"
)

// AdminAuthHandler handles admin authentication requests
type AdminAuthHandler struct {
	store auth.AdminStore
	cfg   *config.Config
}

// NewAdminAuthHandler creates a new admin auth handler
func NewAdminAuthHandler(store auth.AdminStore, cfg *config.Config) *AdminAuthHandler {
	return &AdminAuthHandler{
		store: store,
		cfg:   cfg,
	}
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// TokenAuthRequest represents the token authentication request payload
type TokenAuthRequest struct {
	ServiceName string `json:"service_name"`
	Token       string `json:"token"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	AdminID   string `json:"admin_id"`
	AuthType  string `json:"auth_type"`
}

// Login handles email/password authentication
func (h *AdminAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Email == "" || req.Password == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	token, expiresAt, err := auth.GenerateAdminJWTWithPassword(r.Context(), req.Email, req.Password, h.store, h.cfg)
	if err != nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Get admin ID from token claims
	claims, _ := auth.ValidateAdminJWT(token, h.cfg)

	utils.RespondWithJSON(w, http.StatusOK, AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		AdminID:   claims.AdminID,
		AuthType:  string(claims.AuthType),
	})
}

// TokenAuth handles service token authentication
func (h *AdminAuthHandler) TokenAuth(w http.ResponseWriter, r *http.Request) {
	var req TokenAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.ServiceName == "" || req.Token == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Service name and token are required")
		return
	}

	jwtToken, expiresAt, err := auth.GenerateAdminJWTWithToken(r.Context(), req.ServiceName, req.Token, h.store, h.cfg)
	if err != nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Get admin ID from token claims
	claims, _ := auth.ValidateAdminJWT(jwtToken, h.cfg)

	utils.RespondWithJSON(w, http.StatusOK, AuthResponse{
		Token:     jwtToken,
		ExpiresAt: expiresAt,
		AdminID:   claims.AdminID,
		AuthType:  string(claims.AuthType),
	})
}

// TestProtected is a test endpoint to verify JWT middleware
func (h *AdminAuthHandler) TestProtected(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAdminClaims(r.Context())
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "No admin claims found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Access granted",
		"admin_id":     claims.AdminID,
		"auth_type":    claims.AuthType,
		"roles":        claims.Roles,
		"email":        claims.Email,
		"service_name": claims.ServiceName,
	})
}

// handleAdminKeys serves the key management API (create/revoke/regenerate).
func (d *Dependencies) handleAdminKeys(w http.ResponseWriter, r *http.Request) {
	// TODO: route by method, e.g. POST=create, GET=list, etc.

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "admin keys endpoint not implemented yet",
	})
}
