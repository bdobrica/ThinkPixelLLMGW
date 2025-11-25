package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// Encryption provides AES-GCM encryption/decryption for sensitive data
type Encryption struct {
	key []byte
}

// NewEncryption creates a new encryption service with the given key
// The key should be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256
func NewEncryption(key []byte) (*Encryption, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("invalid key size: must be 16, 24, or 32 bytes, got %d", len(key))
	}

	return &Encryption{
		key: key,
	}, nil
}

// NewEncryptionFromBase64 creates a new encryption service from a base64-encoded key
func NewEncryptionFromBase64(encodedKey string) (*Encryption, error) {
	if encodedKey == "" {
		return nil, fmt.Errorf("encryption key cannot be empty")
	}

	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 key: %w", err)
	}

	return NewEncryption(key)
}

// GenerateKey generates a new random encryption key of the specified size
// Returns the key as a base64-encoded string for easy storage in environment variables
func GenerateKey(keySize int) (string, error) {
	if keySize != 16 && keySize != 24 && keySize != 32 {
		return "", fmt.Errorf("invalid key size: must be 16, 24, or 32 bytes")
	}

	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

// Encrypt encrypts plaintext using AES-GCM and returns the ciphertext as base64
func (e *Encryption) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Return as base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-GCM
func (e *Encryption) Decrypt(ciphertextBase64 string) ([]byte, error) {
	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptJSON encrypts a JSON-serializable map and returns base64 string
func (e *Encryption) EncryptJSON(data map[string]any) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return e.Encrypt(jsonBytes)
}

// DecryptJSON decrypts base64 string to a map
func (e *Encryption) DecryptJSON(ciphertextBase64 string) (map[string]any, error) {
	if ciphertextBase64 == "" {
		return nil, nil
	}

	plaintext, err := e.Decrypt(ciphertextBase64)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return result, nil
}
