package main

import (
	"fmt"
	"log"

	"llm_gateway/internal/models"
	"llm_gateway/internal/storage"
)

// Example demonstrating how to use encryption for provider credentials
func main() {
	// 1. Generate an encryption key (do this once and store securely)
	keyBase64, err := storage.GenerateKey(32) // AES-256
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}
	fmt.Printf("Generated encryption key: %s\n", keyBase64)
	fmt.Println("Store this in your ENCRYPTION_KEY environment variable")
	fmt.Println()

	// 2. In production, load from environment
	// keyBase64 = os.Getenv("ENCRYPTION_KEY")

	// 3. Create encryption service
	encryption, err := storage.NewEncryptionFromBase64(keyBase64)
	if err != nil {
		log.Fatalf("Failed to create encryption: %v", err)
	}

	// 4. Example: Encrypt OpenAI credentials
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

	// 5. This would be stored in the database
	provider := &models.Provider{
		Name:                 "openai",
		DisplayName:          "OpenAI",
		ProviderType:         "openai",
		EncryptedCredentials: encryptedCreds,
		Config:               models.JSONB{"base_url": "https://api.openai.com/v1"},
		Enabled:              true,
	}
	fmt.Printf("\nProvider ready to store: %s\n", provider.DisplayName)
	fmt.Println()

	// 6. Example: Decrypt credentials (as done by ProviderRegistry)
	fmt.Println("=== Decrypting Provider Credentials ===")
	decryptedCreds := make(map[string]string)
	for key, val := range encryptedCreds {
		if strVal, ok := val.(string); ok {
			decrypted, err := encryption.Decrypt(strVal)
			if err != nil {
				log.Fatalf("Failed to decrypt %s: %v", key, err)
			}
			decryptedCreds[key] = string(decrypted)
			fmt.Printf("Decrypted %s: %s\n", key, string(decrypted))
		}
	}

	// 7. Verify decryption worked
	fmt.Println()
	if decryptedCreds["api_key"] == credentials["api_key"] {
		fmt.Println("✓ Encryption/Decryption verified successfully!")
	} else {
		fmt.Println("✗ Decryption failed - values don't match")
	}
}
