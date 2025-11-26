package utils

import (
	"testing"
)

func TestHashString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple string",
			input: "hello world",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "special characters",
			input: "!@#$%^&*()_+-={}[]|:;<>?,./",
		},
		{
			name:  "unicode string",
			input: "Hello 世界 🌍",
		},
		{
			name:  "long string",
			input: "this is a very long string that contains many characters and should still hash correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashString(tt.input)

			// SHA256 produces 64 hex characters
			if len(hash) != 64 {
				t.Errorf("HashString() length = %d, want 64", len(hash))
			}

			// Hash should be consistent
			hash2 := HashString(tt.input)
			if hash != hash2 {
				t.Errorf("HashString() not consistent: first=%s, second=%s", hash, hash2)
			}

			// Verify it only contains hex characters
			for _, c := range hash {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("HashString() contains non-hex character: %c", c)
					break
				}
			}
		})
	}
}

func TestHashStringDifferentInputs(t *testing.T) {
	input1 := "string1"
	input2 := "string2"

	hash1 := HashString(input1)
	hash2 := HashString(input2)

	if hash1 == hash2 {
		t.Error("HashString() produced same hash for different inputs")
	}
}

func TestHashStringCollisionResistance(t *testing.T) {
	// Test that similar strings produce different hashes
	testCases := []struct {
		s1 string
		s2 string
	}{
		{"abc", "abd"},
		{"test", "Test"},
		{"hello", "hello "},
		{"12345", "123456"},
	}

	for _, tc := range testCases {
		hash1 := HashString(tc.s1)
		hash2 := HashString(tc.s2)

		if hash1 == hash2 {
			t.Errorf("HashString() collision for '%s' and '%s'", tc.s1, tc.s2)
		}
	}
}
