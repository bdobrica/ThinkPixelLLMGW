package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminAuthHandler_Login_MethodNotAllowed(t *testing.T) {
	handler := NewAdminAuthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/auth/login", strings.NewReader(`{"email":"a@b.com","password":"x"}`))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Method not allowed" {
		t.Fatalf("expected method not allowed message, got %q", resp["error"])
	}
}

func TestAdminAuthHandler_TokenAuth_MethodNotAllowed(t *testing.T) {
	handler := NewAdminAuthHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/auth/token", strings.NewReader(`{"service_name":"svc","token":"x"}`))
	w := httptest.NewRecorder()

	handler.TokenAuth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Method not allowed" {
		t.Fatalf("expected method not allowed message, got %q", resp["error"])
	}
}

func TestAdminAuthHandler_Login_RequestBodyTooLarge(t *testing.T) {
	handler := NewAdminAuthHandler(nil, nil)

	largePayload := `{"email":"` + strings.Repeat("a", int(maxJSONRequestBodyBytes)) + `@example.com","password":"pw"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/auth/login", strings.NewReader(largePayload))
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Request body too large" {
		t.Fatalf("expected request body too large message, got %q", resp["error"])
	}
}
