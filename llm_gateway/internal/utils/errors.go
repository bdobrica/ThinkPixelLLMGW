package utils

import (
	"strings"
)

// IsRecoverableError checks if an error is recoverable based on predefined criteria.
func IsRecoverableError(err error) bool {
	// Define recoverable errors based on your application's logic
	recoverableErrors := []string{
		"model API returned status",
	}

	for _, recoverable := range recoverableErrors {
		if strings.HasPrefix(err.Error(), recoverable) {
			return true
		}
	}
	return false
}
