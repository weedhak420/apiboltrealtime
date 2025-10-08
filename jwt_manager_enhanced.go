package main

import (
	"container/list"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// JWTKey represents a JWT signing key
type JWTKey struct {
	ID        string          `json:"id"`
	Key       *rsa.PrivateKey `json:"-"`
	PublicKey *rsa.PublicKey  `json:"-"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Active    bool            `json:"active"`
	GraceEnds time.Time       `json:"grace_period_ends"`
}

// JWTKeyManager manages JWT keys with rotation
type JWTKeyManager struct {
	keys        map[string]*JWTKey
	currentKey  *JWTKey
	mu          sync.RWMutex
	keyRotation time.Duration
	keyLifetime time.Duration
}

// TokenRevocationStore handles token revocation
type TokenRevocationStore interface {
	RevokeToken(ctx context.Context, jti string, expiresAt time.Time) error
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)
	CleanupExpiredTokens(ctx context.Context) error
}

// RedisTokenRevocationStore implements token revocation using Redis
type RedisTokenRevocationStore struct {
	redisClient *redis.Client
	namespace   string
}

// MemoryTokenRevocationStore implements token revocation using in-memory storage
type MemoryTokenRevocationStore struct {
	revokedTokens map[string]time.Time
	mu            sync.RWMutex
}

// CachedRevocationStore wraps another TokenRevocationStore with an LRU cache to reduce backend lookups.
type CachedRevocationStore struct {
	backend     TokenRevocationStore
	cache       *LRURevocationCache
	gracePeriod time.Duration
}

// LRURevocationCache provides an in-memory LRU cache for revoked JTIs.
type LRURevocationCache struct {
	capacity int
	mu       sync.Mutex
	entries  map[string]*list.Element
	order    *list.List
}

type cacheEntry struct {
	jti       string
	expiresAt time.Time
}

// NewCachedRevocationStore creates a cached revocation store wrapper.
func NewCachedRevocationStore(backend TokenRevocationStore, capacity int, gracePeriod time.Duration) TokenRevocationStore {
	if backend == nil {
		return nil
	}

	return &CachedRevocationStore{
		backend:     backend,
		cache:       NewLRURevocationCache(capacity),
		gracePeriod: gracePeriod,
	}
}

// NewLRURevocationCache creates a new in-memory LRU cache for revoked tokens.
func NewLRURevocationCache(capacity int) *LRURevocationCache {
	if capacity <= 0 {
		capacity = 512
	}

	return &LRURevocationCache{
		capacity: capacity,
		entries:  make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Add stores a revoked JTI in the cache.
func (c *LRURevocationCache) Add(jti string, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[jti]; ok {
		elem.Value.(*cacheEntry).expiresAt = expiresAt
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(&cacheEntry{jti: jti, expiresAt: expiresAt})
	c.entries[jti] = elem

	if c.order.Len() > c.capacity {
		tail := c.order.Back()
		if tail != nil {
			entry := tail.Value.(*cacheEntry)
			delete(c.entries, entry.jti)
			c.order.Remove(tail)
		}
	}
}

// Get returns whether the JTI is revoked and whether it was found in cache.
func (c *LRURevocationCache) Get(jti string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.entries[jti]; ok {
		entry := elem.Value.(*cacheEntry)
		if time.Now().After(entry.expiresAt) {
			c.order.Remove(elem)
			delete(c.entries, jti)
			return false, false
		}
		c.order.MoveToFront(elem)
		return true, true
	}

	return false, false
}

// Cleanup removes expired cache entries.
func (c *LRURevocationCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for elem := c.order.Back(); elem != nil; {
		prev := elem.Prev()
		entry := elem.Value.(*cacheEntry)
		if now.After(entry.expiresAt) {
			delete(c.entries, entry.jti)
			c.order.Remove(elem)
		}
		elem = prev
	}
}

// RevokeToken stores the revoked JTI in cache and backend.
func (c *CachedRevocationStore) RevokeToken(ctx context.Context, jti string, expiresAt time.Time) error {
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(c.gracePeriod)
	}

	if err := c.backend.RevokeToken(ctx, jti, expiresAt); err != nil {
		return err
	}

	c.cache.Add(jti, expiresAt)
	return nil
}

// IsTokenRevoked checks cache first before delegating to backend.
func (c *CachedRevocationStore) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	if revoked, ok := c.cache.Get(jti); ok {
		return revoked, nil
	}

	revoked, err := c.backend.IsTokenRevoked(ctx, jti)
	if err != nil {
		return false, err
	}

	if revoked {
		c.cache.Add(jti, time.Now().Add(c.gracePeriod))
	}

	return revoked, nil
}

// CleanupExpiredTokens cleans cache and backend.
func (c *CachedRevocationStore) CleanupExpiredTokens(ctx context.Context) error {
	c.cache.Cleanup()
	if c.backend != nil {
		return c.backend.CleanupExpiredTokens(ctx)
	}
	return nil
}

// JWTClaims represents JWT claims
type JWTClaims struct {
	UserID    string `json:"user_id"`
	LoginID   string `json:"login_id"`
	TokenType string `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// EnhancedJWTManager provides enhanced JWT functionality
type EnhancedJWTManager struct {
	keyManager       *JWTKeyManager
	revocationStore  TokenRevocationStore
	config           *JWTConfig
	mu               sync.RWMutex
	validationLeeway time.Duration
	gracePeriod      time.Duration
}

var (
	errTokenRevoked = errors.New("token has been revoked")
	errTokenExpired = errors.New("token has expired")
)

// Global JWT manager
var (
	enhancedJWTManager *EnhancedJWTManager
	jwtMu              sync.Mutex
)

// InitEnhancedJWTManager initializes the enhanced JWT manager
func InitEnhancedJWTManager(config *JWTConfig, revocationStore TokenRevocationStore) (*EnhancedJWTManager, error) {
	jwtMu.Lock()
	defer jwtMu.Unlock()

	if enhancedJWTManager != nil {
		return enhancedJWTManager, nil
	}

	gracePeriod := 5 * time.Minute
	if gp := strings.TrimSpace(os.Getenv("JWT_KEY_GRACE_MINUTES")); gp != "" {
		if parsed, err := time.ParseDuration(gp); err == nil {
			if parsed >= 5*time.Minute {
				gracePeriod = parsed
			}
		} else if minutes, err := strconv.Atoi(gp); err == nil {
			candidate := time.Duration(minutes) * time.Minute
			if candidate >= 5*time.Minute {
				gracePeriod = candidate
			}
		}
	}

	// Create key manager
	keyManager := &JWTKeyManager{
		keys:        make(map[string]*JWTKey),
		keyRotation: 24 * time.Hour,     // Rotate keys every 24 hours
		keyLifetime: 7 * 24 * time.Hour, // Keys live for 7 days
	}

	// Generate initial key
	if err := keyManager.GenerateNewKey(); err != nil {
		return nil, fmt.Errorf("failed to generate initial JWT key: %w", err)
	}

	if revocationStore == nil {
		if redisClient != nil && isProductionEnv() {
			revocationStore = NewRedisTokenRevocationStore(redisClient, "jwt")
		} else if redisClient != nil {
			revocationStore = NewRedisTokenRevocationStore(redisClient, "jwt")
		} else {
			revocationStore = NewMemoryTokenRevocationStore()
		}
	}

	if revocationStore != nil {
		revocationStore = NewCachedRevocationStore(revocationStore, 2048, gracePeriod)
	}

	enhancedJWTManager = &EnhancedJWTManager{
		keyManager:       keyManager,
		revocationStore:  revocationStore,
		config:           config,
		validationLeeway: 2 * time.Minute,
		gracePeriod:      gracePeriod,
	}

	// Start key rotation
	go enhancedJWTManager.startKeyRotation()

	// Start cleanup of expired tokens
	go enhancedJWTManager.startTokenCleanup()

	return enhancedJWTManager, nil
}

// GetEnhancedJWTManager returns the global enhanced JWT manager
func GetEnhancedJWTManager() *EnhancedJWTManager {
	jwtMu.Lock()
	defer jwtMu.Unlock()
	return enhancedJWTManager
}

// GenerateNewKey generates a new JWT key
func (km *JWTKeyManager) GenerateNewKey() error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Create key ID
	keyID := uuid.New().String()

	now := time.Now()

	// Create JWT key
	jwtKey := &JWTKey{
		ID:        keyID,
		Key:       privateKey,
		PublicKey: &privateKey.PublicKey,
		CreatedAt: now,
		ExpiresAt: now.Add(km.keyLifetime),
		Active:    true,
		GraceEnds: now.Add(km.keyLifetime),
	}

	// Store key
	km.keys[keyID] = jwtKey
	km.currentKey = jwtKey

	GetLogger().Info("Generated new JWT key",
		zap.String("key_id", keyID),
		zap.Time("expires_at", jwtKey.ExpiresAt),
	)

	return nil
}

// GetCurrentKey returns the current active key
func (km *JWTKeyManager) GetCurrentKey() *JWTKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentKey
}

// GetKeyByID returns a key by ID
func (km *JWTKeyManager) GetKeyByID(keyID string) *JWTKey {
	km.mu.RLock()
	defer km.mu.RUnlock()
	key := km.keys[keyID]
	if key == nil {
		return nil
	}

	if time.Now().After(key.GraceEnds) {
		return nil
	}

	return key
}

// RotateKey rotates the current key
func (km *JWTKeyManager) RotateKey(gracePeriod time.Duration) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	// Deactivate current key
	if km.currentKey != nil {
		km.currentKey.Active = false
		km.currentKey.GraceEnds = time.Now().Add(gracePeriod)
	}

	// Generate new key
	if err := km.GenerateNewKey(); err != nil {
		return err
	}

	// Clean up old keys
	km.cleanupOldKeys()

	return nil
}

// cleanupOldKeys removes expired keys
func (km *JWTKeyManager) cleanupOldKeys() {
	now := time.Now()
	for keyID, key := range km.keys {
		if now.After(key.GraceEnds) {
			delete(km.keys, keyID)
			GetLogger().Info("Cleaned up expired JWT key", zap.String("key_id", keyID))
		}
	}
}

// startKeyRotation starts the key rotation process
func (ejm *EnhancedJWTManager) startKeyRotation() {
	ticker := time.NewTicker(ejm.keyManager.keyRotation)
	defer ticker.Stop()

	for range ticker.C {
		if err := ejm.keyManager.RotateKey(ejm.gracePeriod); err != nil {
			GetLogger().Error("Failed to rotate JWT key", zap.Error(err))
			continue
		}
		if key := ejm.keyManager.GetCurrentKey(); key != nil {
			GetLogger().Info("Rotated JWT key", zap.String("key_id", key.ID), zap.Time("grace_until", key.GraceEnds))
		}
	}
}

// startTokenCleanup starts the token cleanup process
func (ejm *EnhancedJWTManager) startTokenCleanup() {
	ticker := time.NewTicker(1 * time.Hour) // Cleanup every hour
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if ejm.revocationStore != nil {
			if err := ejm.revocationStore.CleanupExpiredTokens(ctx); err != nil {
				GetLogger().Error("Failed to cleanup expired tokens", zap.Error(err))
			}
		}
		cancel()
	}
}

// GenerateAccessToken generates a new access token
func (ejm *EnhancedJWTManager) GenerateAccessToken(ctx context.Context, userID, loginID string) (string, error) {
	ejm.mu.RLock()
	keyManager := ejm.keyManager
	config := ejm.config
	ejm.mu.RUnlock()

	currentKey := keyManager.GetCurrentKey()
	if currentKey == nil {
		return "", fmt.Errorf("no active JWT key available")
	}

	// Create claims
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		LoginID:   loginID,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    config.Issuer,
			Audience:  []string{config.Audience},
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(config.AccessTokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = currentKey.ID

	// Sign token
	tokenString, err := token.SignedString(currentKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("Generated access token",
			zap.String("user_id", userID),
			zap.String("jti", claims.ID),
			zap.Time("expires_at", claims.ExpiresAt.Time),
		)
	}

	return tokenString, nil
}

// GenerateRefreshToken generates a new refresh token
func (ejm *EnhancedJWTManager) GenerateRefreshToken(ctx context.Context, userID, loginID string) (string, error) {
	ejm.mu.RLock()
	keyManager := ejm.keyManager
	config := ejm.config
	ejm.mu.RUnlock()

	currentKey := keyManager.GetCurrentKey()
	if currentKey == nil {
		return "", fmt.Errorf("no active JWT key available")
	}

	// Create claims
	now := time.Now()
	claims := &JWTClaims{
		UserID:    userID,
		LoginID:   loginID,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    config.Issuer,
			Audience:  []string{config.Audience},
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(config.RefreshTokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = currentKey.ID

	// Sign token
	tokenString, err := token.SignedString(currentKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("Generated refresh token",
			zap.String("user_id", userID),
			zap.String("jti", claims.ID),
			zap.Time("expires_at", claims.ExpiresAt.Time),
		)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token
func (ejm *EnhancedJWTManager) ValidateToken(ctx context.Context, tokenString string) (*JWTClaims, error) {
	ejm.mu.RLock()
	keyManager := ejm.keyManager
	revocationStore := ejm.revocationStore
	config := ejm.config
	leeway := ejm.validationLeeway
	ejm.mu.RUnlock()

	parserOptions := []jwt.ParserOption{
		jwt.WithLeeway(leeway),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	}

	if config != nil {
		if config.Audience != "" {
			parserOptions = append(parserOptions, jwt.WithAudience(config.Audience))
		}
		if config.Issuer != "" {
			parserOptions = append(parserOptions, jwt.WithIssuer(config.Issuer))
		}
	}

	// Parse token and verify signature based on key ID
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}

		keyID, ok := token.Header["kid"].(string)
		if !ok || keyID == "" {
			return nil, fmt.Errorf("missing key ID in token header")
		}

		key := keyManager.GetKeyByID(keyID)
		if key == nil {
			return nil, fmt.Errorf("key not found: %s", keyID)
		}

		return key.PublicKey, nil
	}, parserOptions...)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Get claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time.Add(leeway)) {
		if logger := GetLogger(); logger != nil {
			logger.Warn("Token validation failed - expired",
				zap.String("user_id", claims.UserID),
				zap.String("jti", claims.ID),
				zap.Time("expires_at", claims.ExpiresAt.Time),
			)
		}
		return claims, errTokenExpired
	}

	if revocationStore != nil {
		revoked, err := revocationStore.IsTokenRevoked(ctx, claims.ID)
		if err != nil {
			if logger := GetLogger(); logger != nil {
				logger.Error("Failed to check token revocation",
					zap.Error(err),
					zap.String("user_id", claims.UserID),
					zap.String("jti", claims.ID),
					zap.Time("expires_at", claims.ExpiresAt.Time),
				)
			}
			if metrics := GetMetricsCollector(); metrics != nil {
				metrics.RecordAPIError("jwt_validation", "revocation_lookup_error")
			}
			return nil, fmt.Errorf("revocation check failed: %w", err)
		}
		if revoked {
			if logger := GetLogger(); logger != nil {
				logger.Warn("Token validation failed - revoked",
					zap.String("user_id", claims.UserID),
					zap.String("jti", claims.ID),
					zap.Time("expires_at", claims.ExpiresAt.Time),
				)
			}
			if metrics := GetMetricsCollector(); metrics != nil {
				metrics.RecordAPIError("jwt_validation", "revoked")
			}
			return claims, errTokenRevoked
		}
	}

	return claims, nil
}

// IsTokenRevoked checks if a token with the given JTI has been revoked.
func (ejm *EnhancedJWTManager) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	if ejm == nil || ejm.revocationStore == nil {
		return false, nil
	}
	return ejm.revocationStore.IsTokenRevoked(ctx, jti)
}

// RevokeToken revokes a token
func (ejm *EnhancedJWTManager) RevokeToken(ctx context.Context, tokenString string) error {
	claims, err := ejm.ValidateToken(ctx, tokenString)
	if err != nil {
		switch {
		case errors.Is(err, errTokenRevoked):
			return nil
		case errors.Is(err, errTokenExpired):
			// proceed with revocation using extracted claims even if expired
		default:
			if logger := GetLogger(); logger != nil {
				logger.Error("Failed to validate token before revocation", zap.Error(err))
			}
			if metrics := GetMetricsCollector(); metrics != nil {
				metrics.RecordAPIError("jwt_revocation", "validation_failed")
			}
			return fmt.Errorf("failed to validate token before revocation: %w", err)
		}
	}

	if claims == nil || claims.ID == "" {
		return errors.New("unable to extract claims for revocation")
	}

	if ejm.revocationStore == nil {
		return nil
	}

	expiresAt := time.Now().Add(ejm.gracePeriod)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}

	if err := ejm.revocationStore.RevokeToken(ctx, claims.ID, expiresAt); err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("Failed to revoke token", zap.Error(err), zap.String("user_id", claims.UserID), zap.String("jti", claims.ID), zap.Time("expires_at", expiresAt))
		}
		if metrics := GetMetricsCollector(); metrics != nil {
			metrics.RecordAPIError("jwt_revocation", "backend_error")
		}
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("Token revoked",
			zap.String("user_id", claims.UserID),
			zap.String("jti", claims.ID),
			zap.Time("expires_at", expiresAt),
		)
	}

	return nil
}

// RefreshTokenPair refreshes both access and refresh tokens
func (ejm *EnhancedJWTManager) RefreshTokenPair(ctx context.Context, refreshToken string) (string, string, error) {
	// Validate refresh token
	claims, err := ejm.ValidateToken(ctx, refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// Check if it's a refresh token
	if claims.TokenType != "refresh" {
		return "", "", fmt.Errorf("token is not a refresh token")
	}

	// Revoke old refresh token
	if err := ejm.RevokeToken(ctx, refreshToken); err != nil {
		GetLogger().Error("Failed to revoke old refresh token", zap.Error(err))
	}

	// Generate new token pair
	accessToken, err := ejm.GenerateAccessToken(ctx, claims.UserID, claims.LoginID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := ejm.GenerateRefreshToken(ctx, claims.UserID, claims.LoginID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, newRefreshToken, nil
}

// GetPublicKeys returns all active public keys for token verification
func (ejm *EnhancedJWTManager) GetPublicKeys() map[string]interface{} {
	ejm.mu.RLock()
	keyManager := ejm.keyManager
	ejm.mu.RUnlock()

	keys := make(map[string]interface{})

	keyManager.mu.RLock()
	defer keyManager.mu.RUnlock()

	for keyID, key := range keyManager.keys {
		if key.Active {
			// Convert public key to PEM format
			pubKeyBytes, err := x509.MarshalPKIXPublicKey(key.PublicKey)
			if err != nil {
				GetLogger().Error("Failed to marshal public key", zap.Error(err))
				continue
			}

			pubKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: pubKeyBytes,
			})

			keys[keyID] = map[string]interface{}{
				"kty": "RSA",
				"kid": keyID,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				"pem": string(pubKeyPEM),
			}
		}
	}

	return keys
}

// RedisTokenRevocationStore implementation

// NewRedisTokenRevocationStore creates a new Redis token revocation store
func NewRedisTokenRevocationStore(redisClient *redis.Client, namespace string) *RedisTokenRevocationStore {
	return &RedisTokenRevocationStore{
		redisClient: redisClient,
		namespace:   namespace,
	}
}

// RevokeToken revokes a token in Redis
func (r *RedisTokenRevocationStore) RevokeToken(ctx context.Context, jti string, expiresAt time.Time) error {
	key := fmt.Sprintf("%s:revoked:%s", r.namespace, jti)

	// Set expiration time
	expiration := time.Until(expiresAt)
	if expiration <= 0 {
		return nil // Token already expired
	}

	return r.redisClient.Set(ctx, key, "revoked", expiration).Err()
}

// IsTokenRevoked checks if a token is revoked in Redis
func (r *RedisTokenRevocationStore) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("%s:revoked:%s", r.namespace, jti)

	result := r.redisClient.Exists(ctx, key)
	if result.Err() != nil {
		return false, result.Err()
	}

	return result.Val() > 0, nil
}

// CleanupExpiredTokens cleans up expired tokens from Redis
func (r *RedisTokenRevocationStore) CleanupExpiredTokens(ctx context.Context) error {
	// Redis automatically expires keys, so no manual cleanup needed
	return nil
}

// MemoryTokenRevocationStore implementation

// NewMemoryTokenRevocationStore creates a new memory token revocation store
func NewMemoryTokenRevocationStore() *MemoryTokenRevocationStore {
	return &MemoryTokenRevocationStore{
		revokedTokens: make(map[string]time.Time),
	}
}

// RevokeToken revokes a token in memory
func (m *MemoryTokenRevocationStore) RevokeToken(ctx context.Context, jti string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.revokedTokens[jti] = expiresAt
	return nil
}

// IsTokenRevoked checks if a token is revoked in memory
func (m *MemoryTokenRevocationStore) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	expiresAt, exists := m.revokedTokens[jti]
	if !exists {
		return false, nil
	}

	// Check if token has expired
	if time.Now().After(expiresAt) {
		// Remove expired token
		m.mu.Lock()
		delete(m.revokedTokens, jti)
		m.mu.Unlock()
		return false, nil
	}

	return true, nil
}

// CleanupExpiredTokens cleans up expired tokens from memory
func (m *MemoryTokenRevocationStore) CleanupExpiredTokens(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for jti, expiresAt := range m.revokedTokens {
		if now.After(expiresAt) {
			delete(m.revokedTokens, jti)
		}
	}

	return nil
}

func isProductionEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return env == "prod" || env == "production"
}
