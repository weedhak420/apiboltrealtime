package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// JWTGenerator delegates token issuance to the EnhancedJWTManager.
type JWTGenerator struct {
	manager *EnhancedJWTManager
	userID  string
	loginID string
}

// NewJWTGenerator constructs a generator that issues RS256 tokens through the enhanced manager.
func NewJWTGenerator(manager *EnhancedJWTManager, userID, loginID string) *JWTGenerator {
	return &JWTGenerator{
		manager: manager,
		userID:  userID,
		loginID: loginID,
	}
}

// GenerateToken issues a new access token using the enhanced JWT manager.
func (jg *JWTGenerator) GenerateToken(ctx context.Context) (string, error) {
	if jg == nil || jg.manager == nil {
		return "", errors.New("enhanced jwt manager is not initialized")
	}

	token, err := jg.manager.GenerateAccessToken(ctx, jg.userID, jg.loginID)
	if err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("Failed to generate access token",
				zap.Error(err),
				zap.String("user_id", jg.userID),
				zap.String("login_id", jg.loginID),
			)
		}
		return "", err
	}

	return token, nil
}

// LegacyJWTGenerator retains the old HS256 implementation for testing purposes only.
type LegacyJWTGenerator struct {
	secret  []byte
	userID  int64
	loginID int64
	ttl     time.Duration
}

// NewLegacyJWTGenerator constructs a legacy generator with HS256 signing.
func NewLegacyJWTGenerator(secret string, userID, loginID int64) *LegacyJWTGenerator {
	return &LegacyJWTGenerator{
		secret:  []byte(secret),
		userID:  userID,
		loginID: loginID,
		ttl:     time.Hour,
	}
}

// GenerateToken creates an HS256 token. Use only in tests.
func (lg *LegacyJWTGenerator) GenerateToken() (string, error) {
	if lg == nil || len(lg.secret) == 0 {
		return "", errors.New("legacy jwt generator secret not configured")
	}

	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	now := time.Now()
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"user_id":       lg.userID,
			"user_login_id": lg.loginID,
		},
		"iat": now.Unix(),
		"exp": now.Add(lg.ttl).Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	message := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	signature := lg.sign(message)
	token := message + "." + signature

	if logger := GetLogger(); logger != nil {
		logger.Info("Legacy JWT token generated",
			zap.Int64("user_id", lg.userID),
			zap.Int64("login_id", lg.loginID),
			zap.Time("expires_at", now.Add(lg.ttl)),
			zap.Bool("legacy", true),
		)
	}

	return token, nil
}

func (lg *LegacyJWTGenerator) sign(message string) string {
	h := hmac.New(sha256.New, lg.secret)
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

var (
	jwtGenerator       *JWTGenerator
	legacyJWTGenerator *LegacyJWTGenerator
)

// InitializeJWTGenerator wires the generator to the enhanced manager.
func InitializeJWTGenerator() {
	if jwtGenerator != nil {
		return
	}

	manager := GetEnhancedJWTManager()
	if manager == nil {
		if logger := GetLogger(); logger != nil {
			logger.Warn("Enhanced JWT manager not available for generator")
		}
		return
	}

	userID := strings.TrimSpace(os.Getenv("BOLT_JWT_USER_ID"))
	if userID == "" {
		userID = "283617495"
	}

	loginID := strings.TrimSpace(os.Getenv("BOLT_JWT_LOGIN_ID"))
	if loginID == "" {
		loginID = "605354782"
	}

	jwtGenerator = NewJWTGenerator(manager, userID, loginID)

	if secret := strings.TrimSpace(os.Getenv("LEGACY_JWT_SECRET")); secret != "" {
		uid, _ := strconv.ParseInt(userID, 10, 64)
		lid, _ := strconv.ParseInt(loginID, 10, 64)
		legacyJWTGenerator = NewLegacyJWTGenerator(secret, uid, lid)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("JWT generator initialized",
			zap.String("user_id", userID),
			zap.String("login_id", loginID),
			zap.Bool("legacy_enabled", legacyJWTGenerator != nil),
		)
	}
}

// GenerateNewJWTToken returns a new RS256 access token.
func GenerateNewJWTToken() (string, error) {
	if jwtGenerator == nil {
		InitializeJWTGenerator()
	}
	if jwtGenerator == nil {
		return "", errors.New("jwt generator not initialized")
	}
	return jwtGenerator.GenerateToken(context.Background())
}

// GetLegacyJWTGenerator exposes the HS256 generator for test environments only.
func GetLegacyJWTGenerator() *LegacyJWTGenerator {
	return legacyJWTGenerator
}
