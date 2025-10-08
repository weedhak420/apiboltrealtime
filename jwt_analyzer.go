package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// JWTTokenData represents the legacy JWT token payload structure.
type JWTTokenData struct {
	Data struct {
		UserID      int64 `json:"user_id"`
		UserLoginID int64 `json:"user_login_id"`
	} `json:"data"`
	IAT int64  `json:"iat"`
	EXP int64  `json:"exp"`
	JTI string `json:"jti,omitempty"`

	RawClaims map[string]interface{} `json:"-"`
}

// TokenMetadata captures normalized claim data for analysis and logging.
type TokenMetadata struct {
	RawClaims map[string]interface{}
	UserID    string
	LoginID   string
	JTI       string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Expired   bool
	Revoked   bool
}

// AnalyzeJWTToken decodes the payload into JWTTokenData while retaining raw claims.
func AnalyzeJWTToken(token string) (*JWTTokenData, error) {
	parts := splitJWTToken(token)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JWT payload: %w", err)
	}

	tokenData := &JWTTokenData{RawClaims: raw}

	if data, ok := raw["data"].(map[string]interface{}); ok {
		if v, exists := data["user_id"]; exists {
			tokenData.Data.UserID = parseToInt64(v)
		}
		if v, exists := data["user_login_id"]; exists {
			tokenData.Data.UserLoginID = parseToInt64(v)
		}
	}

	if v, ok := raw["iat"]; ok {
		tokenData.IAT = parseToInt64(v)
	}
	if v, ok := raw["exp"]; ok {
		tokenData.EXP = parseToInt64(v)
	}
	if v, ok := raw["jti"].(string); ok {
		tokenData.JTI = v
	}

	return tokenData, nil
}

// ExtractTokenMetadata normalizes claims and derives expiration status.
func ExtractTokenMetadata(token string) (*TokenMetadata, error) {
	tokenData, err := AnalyzeJWTToken(token)
	if err != nil {
		return nil, err
	}

	metadata := &TokenMetadata{
		RawClaims: tokenData.RawClaims,
		UserID:    normalizeUserID(tokenData),
		LoginID:   normalizeLoginID(tokenData),
		JTI:       tokenData.JTI,
	}

	if metadata.JTI == "" {
		if jti, ok := tokenData.RawClaims["jti"].(string); ok {
			metadata.JTI = jti
		}
	}

	if tokenData.IAT != 0 {
		metadata.IssuedAt = time.Unix(tokenData.IAT, 0)
	} else if v, ok := tokenData.RawClaims["iat"]; ok {
		metadata.IssuedAt = time.Unix(parseToInt64(v), 0)
	}

	if tokenData.EXP != 0 {
		metadata.ExpiresAt = time.Unix(tokenData.EXP, 0)
	} else if v, ok := tokenData.RawClaims["exp"]; ok {
		metadata.ExpiresAt = time.Unix(parseToInt64(v), 0)
	}

	if !metadata.ExpiresAt.IsZero() && time.Now().After(metadata.ExpiresAt) {
		metadata.Expired = true
	}

	return metadata, nil
}

// PrettyPrintClaims returns indented JSON claims for diagnostics.
func PrettyPrintClaims(token string) (string, error) {
	metadata, err := ExtractTokenMetadata(token)
	if err != nil {
		return "", err
	}

	claims, err := json.MarshalIndent(metadata.RawClaims, "", "  ")
	if err != nil {
		return "", err
	}

	return string(claims), nil
}

// AnalyzeAndLogToken extracts metadata, optionally validates signature, and logs results.
func AnalyzeAndLogToken(token string, validateSignature bool) (*TokenMetadata, error) {
	metadata, err := ExtractTokenMetadata(token)
	if err != nil {
		return nil, err
	}

	manager := GetEnhancedJWTManager()
	if manager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if validateSignature {
			claims, err := manager.ValidateToken(ctx, token)
			if err != nil {
				if errors.Is(err, errTokenRevoked) {
					metadata.Revoked = true
				} else if errors.Is(err, errTokenExpired) {
					metadata.Expired = true
				} else if logger := GetLogger(); logger != nil {
					logger.Warn("Token validation failed",
						zap.Error(err),
						zap.String("jti", metadata.JTI),
					)
				}
			} else if claims != nil {
				metadata.UserID = claims.UserID
				metadata.LoginID = claims.LoginID
				metadata.JTI = claims.ID
				metadata.IssuedAt = claims.IssuedAt.Time
				metadata.ExpiresAt = claims.ExpiresAt.Time
				metadata.Expired = time.Now().After(metadata.ExpiresAt)
			}
		} else if metadata.JTI != "" {
			revoked, err := manager.IsTokenRevoked(ctx, metadata.JTI)
			if err != nil {
				if logger := GetLogger(); logger != nil {
					logger.Warn("Failed to check token revocation",
						zap.Error(err),
						zap.String("jti", metadata.JTI),
					)
				}
			} else {
				metadata.Revoked = revoked
			}
		}
	}

	if logger := GetLogger(); logger != nil {
		fields := []zap.Field{
			zap.String("user_id", metadata.UserID),
			zap.String("login_id", metadata.LoginID),
			zap.String("jti", metadata.JTI),
			zap.Time("issued_at", metadata.IssuedAt),
			zap.Time("expires_at", metadata.ExpiresAt),
			zap.Bool("expired", metadata.Expired),
			zap.Bool("revoked", metadata.Revoked),
		}
		logger.Info("JWT token analysis", fields...)
		if metadata.Expired {
			logger.Warn("JWT token has expired", zap.Time("expires_at", metadata.ExpiresAt))
		}
		if metadata.Revoked {
			logger.Warn("JWT token has been revoked", zap.String("jti", metadata.JTI))
		}
	}

	return metadata, nil
}

// LogTokenInfo logs metadata without signature validation.
func LogTokenInfo(token string) {
	LogTokenInfoWithValidation(token, false)
}

// LogTokenInfoWithValidation allows toggling signature validation.
func LogTokenInfoWithValidation(token string, validateSignature bool) {
	if _, err := AnalyzeAndLogToken(token, validateSignature); err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("Failed to analyze JWT token", zap.Error(err))
		}
	}
}

// IsTokenValid checks whether the token has expired.
func IsTokenValid(token string) bool {
	metadata, err := ExtractTokenMetadata(token)
	if err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Warn("Failed to analyze JWT token", zap.Error(err))
		}
		return false
	}

	if metadata.ExpiresAt.IsZero() {
		return true
	}

	return time.Now().Before(metadata.ExpiresAt)
}

// GetTokenExpiry returns the expiry time of a JWT token.
func GetTokenExpiry(token string) (time.Time, error) {
	metadata, err := ExtractTokenMetadata(token)
	if err != nil {
		return time.Time{}, err
	}
	return metadata.ExpiresAt, nil
}

// GetTokenIssuedAt returns the issued time of a JWT token.
func GetTokenIssuedAt(token string) (time.Time, error) {
	metadata, err := ExtractTokenMetadata(token)
	if err != nil {
		return time.Time{}, err
	}
	return metadata.IssuedAt, nil
}

// GetTokenTimeRemaining returns the time remaining until token expires.
func GetTokenTimeRemaining(token string) (time.Duration, error) {
	metadata, err := ExtractTokenMetadata(token)
	if err != nil {
		return 0, err
	}

	if metadata.ExpiresAt.IsZero() {
		return 0, nil
	}

	now := time.Now()
	if now.After(metadata.ExpiresAt) {
		return 0, nil
	}

	return metadata.ExpiresAt.Sub(now), nil
}

// splitJWTToken splits JWT token into header, payload, and signature.
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

func parseToInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		parsed, _ := v.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(v, 10, 64)
		return parsed
	default:
		return 0
	}
}

func normalizeUserID(data *JWTTokenData) string {
	if data.Data.UserID != 0 {
		return strconv.FormatInt(data.Data.UserID, 10)
	}
	if raw, ok := data.RawClaims["user_id"].(string); ok {
		return raw
	}
	if raw, ok := data.RawClaims["sub"].(string); ok {
		return raw
	}
	return ""
}

func normalizeLoginID(data *JWTTokenData) string {
	if data.Data.UserLoginID != 0 {
		return strconv.FormatInt(data.Data.UserLoginID, 10)
	}
	if raw, ok := data.RawClaims["login_id"].(string); ok {
		return raw
	}
	return ""
}
