package providers

import "testing"

func TestCredentialOverridesFromEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "runtime-openai-secret")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("VERTEX_AI_ACCESS_TOKEN", "runtime-google-token")
	t.Setenv("VERTEX_AI_SERVICE_ACCOUNT_JSON", "")
	overrides := CredentialOverridesFromEnvironment()
	if got := overrides["openai"]["api_key"]; got != "runtime-openai-secret" {
		t.Fatalf("openai override = %q", got)
	}
	if _, exists := overrides["anthropic"]; exists {
		t.Fatal("empty value must not create an override")
	}
	if got := overrides["vertexai"]["access_token"]; got != "runtime-google-token" {
		t.Fatalf("vertexai override = %q", got)
	}
}

func TestCredentialKeyNamesNeverReturnsValues(t *testing.T) {
	keys := credentialKeyNames(map[string]string{"api_key": "never-log-this", "token": "or-this"})
	if len(keys) != 2 || keys[0] != "api_key" || keys[1] != "token" {
		t.Fatalf("keys = %v", keys)
	}
}
