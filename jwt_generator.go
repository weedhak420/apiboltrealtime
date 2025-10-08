package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// JWTGenerator generates valid JWT tokens
type JWTGenerator struct {
	secretKey string
	userID    int64
	loginID   int64
}

// NewJWTGenerator creates a new JWT generator
func NewJWTGenerator(secretKey string, userID, loginID int64) *JWTGenerator {
	return &JWTGenerator{
		secretKey: secretKey,
		userID:    userID,
		loginID:   loginID,
	}
}

// GenerateToken generates a new JWT token
func (jg *JWTGenerator) GenerateToken() (string, error) {
	// Create header
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}

	// Create payload
	now := time.Now()
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"user_id":       jg.userID,
			"user_login_id": jg.loginID,
		},
		"iat": now.Unix(),
		"exp": now.Add(1 * time.Hour).Unix(), // 1 hour expiry
	}

	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %v", err)
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode payload
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %v", err)
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature
	message := headerEncoded + "." + payloadEncoded
	signature := jg.createSignature(message)

	// Combine all parts
	token := message + "." + signature

	log.Printf("🔑 Generated new JWT token with expiry: %s", now.Add(1*time.Hour).Format(time.RFC3339))
	return token, nil
}

// createSignature creates HMAC-SHA256 signature
func (jg *JWTGenerator) createSignature(message string) string {
	h := hmac.New(sha256.New, []byte(jg.secretKey))
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// Global JWT generator
var jwtGenerator *JWTGenerator

// InitializeJWTGenerator initializes the global JWT generator
func InitializeJWTGenerator() {
	// Use a secret key (in production, this should be from environment variables)
	secretKey := "your-secret-key-here"
	jwtGenerator = NewJWTGenerator(secretKey, 283617495, 605354782)
	log.Println("✅ JWT Generator initialized")
}

// GenerateNewJWTToken generates a new JWT token
func GenerateNewJWTToken() (string, error) {
	if jwtGenerator == nil {
		InitializeJWTGenerator()
	}
	return jwtGenerator.GenerateToken()
}
