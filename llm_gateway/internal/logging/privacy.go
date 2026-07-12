package logging

import (
	"encoding/json"
	"regexp"
	"strings"
)

var credentialPattern = regexp.MustCompile(`(?i)(bearer\s+|sk-)[a-z0-9._-]+`)

func redactCredentialPatterns(value string) string {
	return credentialPattern.ReplaceAllString(value, "[REDACTED]")
}

type PrivacyPolicy struct {
	BodyMode        string
	MaxBodyBytes    int
	SampleRate      float64
	SensitiveFields map[string]struct{}
}

func DefaultPrivacyPolicy() PrivacyPolicy {
	return PrivacyPolicy{BodyMode: "hash", MaxBodyBytes: 4096, SampleRate: 1, SensitiveFields: sensitiveFieldSet(nil)}
}

func NewPrivacyPolicy(mode string, maxBytes int, sampleRate float64, extraFields []string) PrivacyPolicy {
	return PrivacyPolicy{BodyMode: mode, MaxBodyBytes: maxBytes, SampleRate: sampleRate, SensitiveFields: sensitiveFieldSet(extraFields)}
}

func sensitiveFieldSet(extra []string) map[string]struct{} {
	fields := []string{"api_key", "apikey", "access_token", "authorization", "cookie", "password", "secret", "token"}
	result := make(map[string]struct{}, len(fields)+len(extra))
	for _, field := range append(fields, extra...) {
		result[strings.ToLower(strings.TrimSpace(field))] = struct{}{}
	}
	return result
}

func redactJSONValue(value any, sensitive map[string]struct{}) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, found := sensitive[strings.ToLower(key)]; found {
				typed[key] = "[REDACTED]"
			} else {
				typed[key] = redactJSONValue(child, sensitive)
			}
		}
	case []any:
		for i, child := range typed {
			typed[i] = redactJSONValue(child, sensitive)
		}
	}
	return value
}

func sanitizePayload(value any, policy PrivacyPolicy) any {
	if policy.BodyMode != "redacted" || value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	if len(data) > policy.MaxBodyBytes {
		return sanitizeBodyForLogging(data)
	}
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return sanitizeBodyForLogging(data)
	}
	return redactJSONValue(decoded, policy.SensitiveFields)
}
