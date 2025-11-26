package auth

import (
	"llm_gateway/internal/config"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func getTestConfig() *config.Config {
	return &config.Config{
		JWTSecret: []byte("test-secret-key-for-testing"),
	}
}

func TestGenerateJWT(t *testing.T) {
	cfg := getTestConfig()
	apiKey := "test-api-key"
	hashedKey := HashPassword(apiKey)

	token, expirationTime, err := GenerateJWT(apiKey, hashedKey, cfg)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v, want nil", err)
	}

	if token == "" {
		t.Error("GenerateJWT() returned empty token")
	}

	if expirationTime <= time.Now().Unix() {
		t.Error("GenerateJWT() expiration time is in the past")
	}

	// Expiration should be approximately 15 minutes from now
	expectedExpiration := time.Now().Add(15 * time.Minute).Unix()
	if expirationTime < expectedExpiration-5 || expirationTime > expectedExpiration+5 {
		t.Errorf("GenerateJWT() expiration = %v, want approximately %v", expirationTime, expectedExpiration)
	}
}

func TestValidateJWT(t *testing.T) {
	cfg := getTestConfig()
	apiKey := "test-api-key"
	hashedKey := HashPassword(apiKey)

	t.Run("valid token", func(t *testing.T) {
		token, _, err := GenerateJWT(apiKey, hashedKey, cfg)
		if err != nil {
			t.Fatalf("GenerateJWT() error = %v", err)
		}

		parsedToken, err := ValidateJWT(token, cfg)
		if err != nil {
			t.Fatalf("ValidateJWT() error = %v, want nil", err)
		}
		if parsedToken == nil {
			t.Fatalf("ValidateJWT() returned nil token")
		}
		if !parsedToken.Valid {
			t.Error("ValidateJWT() token.Valid = false, want true")
		}
	})

	t.Run("invalid token format", func(t *testing.T) {
		_, err := ValidateJWT("invalid-token-string", cfg)
		if err == nil {
			t.Error("ValidateJWT() error = nil, want error")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := ValidateJWT("", cfg)
		if err == nil {
			t.Error("ValidateJWT() error = nil, want error")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// Create an expired token manually
		expirationTime := time.Now().Add(-1 * time.Hour).Unix()
		claims := jwt.MapClaims{
			"sub":        apiKey,
			"hashed_key": hashedKey,
			"exp":        expirationTime,
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signedToken, err := token.SignedString(cfg.JWTSecret)
		if err != nil {
			t.Fatalf("Failed to create expired token: %v", err)
		}

		parsedToken, err := ValidateJWT(signedToken, cfg)
		if err == nil {
			t.Error("ValidateJWT() error = nil for expired token, want error")
		}
		if parsedToken != nil && parsedToken.Valid {
			t.Error("ValidateJWT() token.Valid = true for expired token, want false")
		}
	})
}

func TestDecodeJWT(t *testing.T) {
	cfg := getTestConfig()
	apiKey := "test-api-key"
	hashedKey := HashPassword(apiKey)

	t.Run("valid token", func(t *testing.T) {
		token, _, err := GenerateJWT(apiKey, hashedKey, cfg)
		if err != nil {
			t.Fatalf("GenerateJWT() error = %v", err)
		}

		decodedHash, err := DecodeJWT(token, cfg)
		if err != nil {
			t.Errorf("DecodeJWT() error = %v, want nil", err)
		}
		if decodedHash != hashedKey {
			t.Errorf("DecodeJWT() = %v, want %v", decodedHash, hashedKey)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := DecodeJWT("invalid-token", cfg)
		if err == nil {
			t.Error("DecodeJWT() error = nil, want error")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := DecodeJWT("", cfg)
		if err == nil {
			t.Error("DecodeJWT() error = nil, want error")
		}
	})

	t.Run("token with wrong signature", func(t *testing.T) {
		// Create a token with a different secret
		wrongSecret := []byte("wrong-secret")
		claims := jwt.MapClaims{
			"sub":        apiKey,
			"hashed_key": hashedKey,
			"exp":        time.Now().Add(15 * time.Minute).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signedToken, err := token.SignedString(wrongSecret)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}

		_, err = DecodeJWT(signedToken, cfg)
		if err == nil {
			t.Error("DecodeJWT() error = nil for wrong signature, want error")
		}
	})
}

func TestJWTRoundTrip(t *testing.T) {
	// Test the full cycle: generate -> validate -> decode
	cfg := getTestConfig()
	apiKey := "round-trip-key"
	hashedKey := HashPassword(apiKey)

	// Generate
	token, expTime, err := GenerateJWT(apiKey, hashedKey, cfg)
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	// Validate
	parsedToken, err := ValidateJWT(token, cfg)
	if err != nil {
		t.Fatalf("ValidateJWT() error = %v", err)
	}
	if !parsedToken.Valid {
		t.Fatal("ValidateJWT() token is not valid")
	}

	// Decode
	decodedHash, err := DecodeJWT(token, cfg)
	if err != nil {
		t.Fatalf("DecodeJWT() error = %v", err)
	}
	if decodedHash != hashedKey {
		t.Errorf("DecodeJWT() = %v, want %v", decodedHash, hashedKey)
	}

	// Verify claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("Failed to parse claims")
	}

	if claims["sub"] != apiKey {
		t.Errorf("Claims sub = %v, want %v", claims["sub"], apiKey)
	}
	if claims["hashed_key"] != hashedKey {
		t.Errorf("Claims hashed_key = %v, want %v", claims["hashed_key"], hashedKey)
	}

	// Verify expiration is approximately correct
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatal("Claims exp is not a float64")
	}
	if int64(exp) != expTime {
		t.Errorf("Claims exp = %v, want %v", int64(exp), expTime)
	}
}
