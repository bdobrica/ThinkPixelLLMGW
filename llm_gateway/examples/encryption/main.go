package main

import (
	"fmt"
	"log"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
)

// This example demonstrates provider credential encryption.
func main() {
	keyBase64, err := storage.GenerateKey(32)
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}
	fmt.Printf("Generated encryption key: %s\n", keyBase64)
	fmt.Println("Store this in your ENCRYPTION_KEY environment variable")
	fmt.Println()

	encryption, err := storage.NewEncryptionFromBase64(keyBase64)
	if err != nil {
		log.Fatalf("Failed to create encryption: %v", err)
	}

	fmt.Println("=== Encrypting Provider Credentials ===")
	credentials := map[string]string{
		"api_key":      "sk-1234567890abcdef",
		"organization": "org-xyz",
	}

	encryptedCreds := make(models.JSONB)
	for key, value := range credentials {
		encrypted, err := encryption.Encrypt([]byte(value))
		if err != nil {
			log.Fatalf("Failed to encrypt %s: %v", key, err)
		}
		encryptedCreds[key] = encrypted
		fmt.Printf("Encrypted %s: %s...\n", key, encrypted[:40])
	}

	provider := &models.Provider{
		Name:                 "openai",
		DisplayName:          "OpenAI",
		ProviderType:         "openai",
		EncryptedCredentials: encryptedCreds,
		Config:               models.JSONB{"base_url": "https://api.openai.com/v1"},
		Enabled:              true,
	}
	fmt.Printf("\nProvider ready to store: %s\n\n", provider.DisplayName)

	fmt.Println("=== Decrypting Provider Credentials ===")
	decryptedCreds := make(map[string]string)
	for key, value := range encryptedCreds {
		stringValue, ok := value.(string)
		if !ok {
			continue
		}
		decrypted, err := encryption.Decrypt(stringValue)
		if err != nil {
			log.Fatalf("Failed to decrypt %s: %v", key, err)
		}
		decryptedCreds[key] = string(decrypted)
		fmt.Printf("Decrypted %s: %s\n", key, decrypted)
	}

	fmt.Println()
	if decryptedCreds["api_key"] == credentials["api_key"] {
		fmt.Println("Encryption/decryption verified successfully")
		return
	}
	log.Fatal("Decryption failed: values do not match")
}
