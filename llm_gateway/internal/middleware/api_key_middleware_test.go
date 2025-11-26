package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm_gateway/internal/auth"
)

func TestAPIKeyMiddleware_Success(t *testing.T) {
	store := auth.NewInMemoryAPIKeyStore()
	middleware := APIKeyMiddleware(store)

	// Create a test handler that the middleware will wrap
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the API key record is in context
		record, ok := GetAPIKeyRecord(r.Context())
		if !ok {
			t.Error("API key record not found in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if record.ID != "demo-key-id" {
			t.Errorf("Unexpected API key ID: %s", record.ID)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := middleware(nextHandler)

	t.Run("with X-API-Key header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("X-API-Key", "demo-key")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("with Bearer token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer demo-key")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})
}

func TestAPIKeyMiddleware_MissingKey(t *testing.T) {
	store := auth.NewInMemoryAPIKeyStore()
	middleware := APIKeyMiddleware(store)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called when API key is missing")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Expected error message in response body")
	}
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	store := auth.NewInMemoryAPIKeyStore()
	middleware := APIKeyMiddleware(store)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called for invalid API key")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-API-Key", "invalid-key-12345")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_RevokedKey(t *testing.T) {
	// Note: The InMemoryAPIKeyStore doesn't expose key modification
	// In a real scenario, you'd use a mock store to test revoked keys
	t.Skip("Skipping revoked key test - requires mock store or exposed key modification")
}

func TestGetAPIKeyRecord(t *testing.T) {
	t.Run("record exists in context", func(t *testing.T) {
		record := &auth.APIKeyRecord{
			ID:            "test-id",
			Name:          "Test Key",
			AllowedModels: []string{"gpt-4"},
		}

		ctx := context.WithValue(context.Background(), APIKeyRecordKey, record)

		retrieved, ok := GetAPIKeyRecord(ctx)
		if !ok {
			t.Error("Expected to find API key record in context")
		}
		if retrieved.ID != "test-id" {
			t.Errorf("Expected ID 'test-id', got '%s'", retrieved.ID)
		}
	})

	t.Run("record not in context", func(t *testing.T) {
		ctx := context.Background()

		_, ok := GetAPIKeyRecord(ctx)
		if ok {
			t.Error("Expected not to find API key record in empty context")
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), APIKeyRecordKey, "not-a-record")

		_, ok := GetAPIKeyRecord(ctx)
		if ok {
			t.Error("Expected type assertion to fail for wrong type")
		}
	})
}

func TestAPIKeyMiddleware_BearerTokenParsing(t *testing.T) {
	store := auth.NewInMemoryAPIKeyStore()
	middleware := APIKeyMiddleware(store)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "valid Bearer token",
			authHeader:     "Bearer demo-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Bearer with no token",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "malformed Bearer",
			authHeader:     "Bearerdemo-key",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "different auth scheme",
			authHeader:     "Basic abc123",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)

			req := httptest.NewRequest("GET", "/api/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
