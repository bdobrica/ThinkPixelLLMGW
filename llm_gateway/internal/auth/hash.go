package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashPassword hashes a password using SHA256
// NOTE: For production use, consider using bcrypt or argon2
func HashPassword(password string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hex.EncodeToString(hasher.Sum(nil))
}
