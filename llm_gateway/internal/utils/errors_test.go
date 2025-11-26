package utils

import (
	"errors"
	"testing"
)

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "recoverable error - model API status",
			err:      errors.New("model API returned status 503"),
			expected: true,
		},
		{
			name:     "recoverable error - model API different code",
			err:      errors.New("model API returned status 429"),
			expected: true,
		},
		{
			name:     "non-recoverable error",
			err:      errors.New("authentication failed"),
			expected: false,
		},
		{
			name:     "non-recoverable error - database",
			err:      errors.New("database connection failed"),
			expected: false,
		},
		{
			name:     "non-recoverable error - validation",
			err:      errors.New("invalid input"),
			expected: false,
		},
		{
			name:     "empty error message",
			err:      errors.New(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRecoverableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsRecoverableErrorEdgeCases(t *testing.T) {
	t.Run("prefix match only", func(t *testing.T) {
		// Should match because it starts with "model API returned status"
		err := errors.New("model API returned status: unexpected response")
		if !IsRecoverableError(err) {
			t.Error("IsRecoverableError() should return true for prefix match")
		}
	})

	t.Run("case sensitive match", func(t *testing.T) {
		// Should NOT match because of different case
		err := errors.New("Model API returned status 503")
		if IsRecoverableError(err) {
			t.Error("IsRecoverableError() should be case sensitive")
		}
	})

	t.Run("substring but not prefix", func(t *testing.T) {
		// Should NOT match because it's not a prefix
		err := errors.New("error: model API returned status 503")
		if IsRecoverableError(err) {
			t.Error("IsRecoverableError() should only match prefixes")
		}
	})
}
