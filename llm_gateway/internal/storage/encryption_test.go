package storage

import (
	"encoding/base64"
	"testing"
)

func TestEncryption(t *testing.T) {
	// Generate a 32-byte key (AES-256)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewEncryption(key)
	if err != nil {
		t.Fatalf("Failed to create encryption: %v", err)
	}

	// Test string encryption/decryption
	plaintext := []byte("my-secret-api-key-12345")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match original. Got %s, want %s", decrypted, plaintext)
	}
}

func TestEncryptionFromBase64(t *testing.T) {
	// Generate a key
	keyBase64, err := GenerateKey(32)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	enc, err := NewEncryptionFromBase64(keyBase64)
	if err != nil {
		t.Fatalf("Failed to create encryption from base64: %v", err)
	}

	// Test encryption/decryption
	plaintext := []byte("test-data")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match original")
	}
}

func TestEncryptJSON(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryption(key)

	// Test JSON encryption/decryption
	data := map[string]any{
		"api_key":    "sk-1234567890",
		"secret_key": "secret-abc-def",
		"region":     "us-east-1",
	}

	ciphertext, err := enc.EncryptJSON(data)
	if err != nil {
		t.Fatalf("Failed to encrypt JSON: %v", err)
	}

	decrypted, err := enc.DecryptJSON(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt JSON: %v", err)
	}

	if decrypted["api_key"] != data["api_key"] {
		t.Errorf("Decrypted JSON doesn't match original")
	}
}

func TestGenerateKey(t *testing.T) {
	// Test AES-256 (32 bytes)
	key, err := GenerateKey(32)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("Generated key is not valid base64: %v", err)
	}

	if len(decoded) != 32 {
		t.Errorf("Generated key has wrong length. Got %d, want 32", len(decoded))
	}

	// Test that we can use the generated key
	enc, err := NewEncryptionFromBase64(key)
	if err != nil {
		t.Fatalf("Failed to create encryption with generated key: %v", err)
	}

	plaintext := []byte("test")
	ciphertext, _ := enc.Encrypt(plaintext)
	decrypted, _ := enc.Decrypt(ciphertext)

	if string(decrypted) != string(plaintext) {
		t.Errorf("Encryption with generated key failed")
	}
}

func TestInvalidKeySize(t *testing.T) {
	// Test invalid key size
	_, err := NewEncryption([]byte("too-short"))
	if err == nil {
		t.Error("Expected error for invalid key size")
	}

	// Test invalid key size for GenerateKey
	_, err = GenerateKey(20)
	if err == nil {
		t.Error("Expected error for invalid key size in GenerateKey")
	}
}

func TestEmptyData(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryption(key)

	// Test empty JSON
	ciphertext, err := enc.EncryptJSON(map[string]any{})
	if err != nil {
		t.Fatalf("Failed to encrypt empty JSON: %v", err)
	}
	if ciphertext != "" {
		t.Errorf("Expected empty ciphertext for empty JSON")
	}

	// Test decrypt empty string
	decrypted, err := enc.DecryptJSON("")
	if err != nil {
		t.Fatalf("Failed to decrypt empty string: %v", err)
	}
	if decrypted != nil {
		t.Errorf("Expected nil for empty ciphertext")
	}
}
