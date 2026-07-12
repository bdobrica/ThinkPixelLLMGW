package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsBearerTokenAuth(t *testing.T) {
	handler := bearerTokenAuth("0123456789abcdef0123456789abcdef", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tt := range []struct {
		name, authorization string
		want                int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic 0123456789abcdef0123456789abcdef", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"authorized", "Bearer 0123456789abcdef0123456789abcdef", http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			req.Header.Set("Authorization", tt.authorization)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
		})
	}
}
