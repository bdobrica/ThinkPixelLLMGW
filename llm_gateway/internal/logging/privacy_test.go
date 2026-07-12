package logging

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestPrivacyPolicyRedactsNestedSensitiveFields(t *testing.T) {
	policy := NewPrivacyPolicy("redacted", 4096, 1, []string{"customer_secret"})
	got := sanitizePayload(map[string]any{
		"messages": []any{map[string]any{"content": "allowed"}},
		"password": "never-log-this",
		"nested":   map[string]any{"customer_secret": "nor-this"},
	}, policy)
	text := mustJSON(t, got)
	for _, secret := range []string{"never-log-this", "nor-this"} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret leaked: %s", text)
		}
	}
	if !strings.Contains(text, "allowed") {
		t.Fatalf("non-sensitive content unexpectedly removed: %s", text)
	}
}

func TestPrivacyPolicyRedactsCredentialsOutsideJSON(t *testing.T) {
	if got := redactCredentialPatterns("upstream said Bearer abc.def and sk-secret123"); strings.Contains(got, "abc.def") || strings.Contains(got, "secret123") {
		t.Fatalf("credential leaked: %s", got)
	}
	req, _ := http.NewRequest("GET", "https://example.test/path?api_key=query-secret&safe=yes", nil)
	got := sanitizeURL(req, DefaultPrivacyPolicy())
	if strings.Contains(got, "query-secret") || !strings.Contains(got, "safe=yes") {
		t.Fatalf("URL sanitization = %s", got)
	}
}

func TestOversizedRedactedPayloadFallsBackToHash(t *testing.T) {
	got := sanitizePayload(map[string]any{"content": strings.Repeat("x", 100)}, NewPrivacyPolicy("redacted", 10, 1, nil))
	if value, ok := got.(string); !ok || !strings.HasPrefix(value, "sha256:") {
		t.Fatalf("got %#v", got)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
