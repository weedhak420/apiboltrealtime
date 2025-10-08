package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// JWTTokenData represents the JWT token data structure
type JWTTokenData struct {
	Data struct {
		UserID      int64 `json:"user_id"`
		UserLoginID int64 `json:"user_login_id"`
	} `json:"data"`
	IAT int64 `json:"iat"` // Issued at
	EXP int64 `json:"exp"` // Expires at
}

// AnalyzeJWTToken analyzes a JWT token and extracts information
func AnalyzeJWTToken(token string) (*JWTTokenData, error) {
	// Split JWT token into parts
	parts := splitJWTToken(token)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %v", err)
	}

	// Parse JSON payload
	var tokenData JWTTokenData
	if err := json.Unmarshal(payload, &tokenData); err != nil {
		return nil, fmt.Errorf("failed to parse JWT payload: %v", err)
	}

	return &tokenData, nil
}

// splitJWTToken splits JWT token into header, payload, and signature
func splitJWTToken(token string) []string {
	parts := make([]string, 0, 3)
	start := 0

	for i, char := range token {
		if char == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}

	if start < len(token) {
		parts = append(parts, token[start:])
	}

	return parts
}

// IsTokenValid checks if a JWT token is still valid
func IsTokenValid(token string) bool {
	tokenData, err := AnalyzeJWTToken(token)
	if err != nil {
		log.Printf("⚠️ Failed to analyze JWT token: %v", err)
		return false
	}

	now := time.Now().Unix()
	return now < tokenData.EXP
}

// GetTokenExpiry returns the expiry time of a JWT token
func GetTokenExpiry(token string) (time.Time, error) {
	tokenData, err := AnalyzeJWTToken(token)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(tokenData.EXP, 0), nil
}

// GetTokenIssuedAt returns the issued time of a JWT token
func GetTokenIssuedAt(token string) (time.Time, error) {
	tokenData, err := AnalyzeJWTToken(token)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(tokenData.IAT, 0), nil
}

// GetTokenTimeRemaining returns the time remaining until token expires
func GetTokenTimeRemaining(token string) (time.Duration, error) {
	expiry, err := GetTokenExpiry(token)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	if now.After(expiry) {
		return 0, nil // Token is expired
	}

	return expiry.Sub(now), nil
}

// LogTokenInfo logs detailed information about a JWT token
func LogTokenInfo(token string) {
	tokenData, err := AnalyzeJWTToken(token)
	if err != nil {
		log.Printf("⚠️ Failed to analyze JWT token: %v", err)
		return
	}

	issuedAt := time.Unix(tokenData.IAT, 0)
	expiresAt := time.Unix(tokenData.EXP, 0)
	now := time.Now()

	log.Printf("🔍 JWT Token Analysis:")
	log.Printf("   User ID: %d", tokenData.Data.UserID)
	log.Printf("   Login ID: %d", tokenData.Data.UserLoginID)
	log.Printf("   Issued At: %s", issuedAt.Format(time.RFC3339))
	log.Printf("   Expires At: %s", expiresAt.Format(time.RFC3339))
	log.Printf("   Current Time: %s", now.Format(time.RFC3339))

	if now.Before(expiresAt) {
		remaining := expiresAt.Sub(now)
		log.Printf("   Time Remaining: %s", remaining.String())
		log.Printf("   Status: ✅ Valid")
	} else {
		log.Printf("   Status: ❌ Expired")
	}
}
