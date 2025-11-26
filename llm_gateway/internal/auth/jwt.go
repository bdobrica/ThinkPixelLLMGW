package auth

import (
	"errors"
	"time"

	"llm_gateway/internal/config"

	"github.com/golang-jwt/jwt/v4"
)

// GenerateJWT creates a short-lived token with the API Key ID embedded
func GenerateJWT(apiKey string, hashedKey string, cfg *config.Config) (string, int64, error) {
	expirationTime := time.Now().Add(15 * time.Minute).Unix()
	claims := jwt.MapClaims{
		"sub":        apiKey,         // Subject: API Key
		"hashed_key": hashedKey,      // Include the API Key Hash
		"exp":        expirationTime, // Expiration timestamp
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(cfg.JWTSecret)
	if err != nil {
		return "", 0, err
	}
	return signedToken, expirationTime, nil
}

// ValidateJWT verifies the provided JWT
func ValidateJWT(tokenString string, cfg *config.Config) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return cfg.JWTSecret, nil
	})
}

// DecodeJWT extracts the API Key ID from the provided JWT
func DecodeJWT(tokenString string, cfg *config.Config) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return cfg.JWTSecret, nil
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
