package auth

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Use a secure method for managing secrets
var jwtSecret = []byte(getEnv("JWT_SECRET", "supersecretkey"))

// getEnv retrieves environment variable or returns default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GenerateJWT creates a short-lived token with the API Key ID embedded
func GenerateJWT(apiKey string, hashedKey string) (string, int64, error) {
	expirationTime := time.Now().Add(15 * time.Minute).Unix()
	claims := jwt.MapClaims{
		"sub":        apiKey,         // Subject: API Key
		"hashed_key": hashedKey,      // Include the API Key Hash
		"exp":        expirationTime, // Expiration timestamp
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}
	return signedToken, expirationTime, nil
}

// ValidateJWT verifies the provided JWT
func ValidateJWT(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
}

// DecodeJWT extracts the API Key ID from the provided JWT
func DecodeJWT(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		hashedKey := claims["hashed_key"].(string)
		return hashedKey, nil
	}
	return "", errors.New("invalid token")
}
