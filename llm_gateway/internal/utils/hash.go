package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashString(s string) string {
	hasher := sha256.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}
