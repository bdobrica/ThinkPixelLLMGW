package providers

import "os"

// CredentialOverridesFromEnvironment returns runtime-only provider credentials.
// Values are intentionally kept out of logs and are never persisted.
func CredentialOverridesFromEnvironment() map[string]map[string]string {
	mappings := []struct{ env, provider, key string }{
		{"OPENAI_API_KEY", "openai", "api_key"},
		{"ANTHROPIC_API_KEY", "anthropic", "api_key"},
	}
	overrides := make(map[string]map[string]string)
	for _, mapping := range mappings {
		value := os.Getenv(mapping.env)
		if value == "" {
			continue
		}
		if overrides[mapping.provider] == nil {
			overrides[mapping.provider] = make(map[string]string)
		}
		overrides[mapping.provider][mapping.key] = value
	}
	return overrides
}
