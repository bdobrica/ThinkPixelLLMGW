package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"llm_gateway/internal/auth"
	"llm_gateway/internal/config"
	"llm_gateway/internal/metrics"
	"llm_gateway/internal/middleware"
)

func TestAdminAuthHandler_CurrentAdmin(t *testing.T) {
	handler := NewAdminAuthHandler(nil, nil)
	claims := &auth.AdminClaims{
		AdminID:  "admin-123",
		AuthType: auth.AdminAuthTypeUser,
		Roles:    []string{"admin", "viewer"},
		Email:    "admin@example.test",
	}
	ctx := context.WithValue(context.Background(), middleware.AdminClaimsKey, claims)
	req := httptest.NewRequest(http.MethodGet, "/admin/me", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.CurrentAdmin(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	var response CurrentAdminResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.AdminID != claims.AdminID || response.AuthType != claims.AuthType || response.Email != claims.Email {
		t.Fatalf("unexpected identity response: %+v", response)
	}
	if !reflect.DeepEqual(response.Roles, claims.Roles) {
		t.Fatalf("roles = %v, want %v", response.Roles, claims.Roles)
	}
	if response.ServiceName != "" {
		t.Fatalf("service_name = %q, want omitted/empty", response.ServiceName)
	}
	var raw map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if _, exists := raw["message"]; exists {
		t.Fatal("stable identity schema must not include legacy test message")
	}
}

func TestAdminAuthHandler_CurrentAdmin_MethodNotAllowed(t *testing.T) {
	handler := NewAdminAuthHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/me", nil)
	recorder := httptest.NewRecorder()

	handler.CurrentAdmin(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow = %q, want %q", allow, http.MethodGet)
	}
}

func TestAdminAuthHandler_CurrentAdmin_RequiresClaims(t *testing.T) {
	handler := NewAdminAuthHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/me", nil)
	recorder := httptest.NewRecorder()

	handler.CurrentAdmin(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestAdminMeRoute_MethodContractAndLegacyRemoval(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, &Dependencies{Metrics: metrics.NewNoopMetrics()}, &config.Config{JWTSecret: []byte("test-secret")})

	postRecorder := httptest.NewRecorder()
	mux.ServeHTTP(postRecorder, httptest.NewRequest(http.MethodPost, "/admin/me", nil))
	if postRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /admin/me status = %d, want %d", postRecorder.Code, http.StatusMethodNotAllowed)
	}
	if allow := postRecorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow = %q, want %q", allow, http.MethodGet)
	}

	legacyRecorder := httptest.NewRecorder()
	mux.ServeHTTP(legacyRecorder, httptest.NewRequest(http.MethodGet, "/admin/test", nil))
	if legacyRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET /admin/test status = %d, want %d", legacyRecorder.Code, http.StatusNotFound)
	}
}
