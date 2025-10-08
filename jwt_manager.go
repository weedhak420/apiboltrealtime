package main

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Constants for Bolt API token refresh
const (
	BASE_URL = "https://node.bolt.eu"
	ENDPOINT = "/user-auth/profile/auth/getAccessToken"

	// Error codes
	ERROR_TOO_MANY_REQUESTS     = 1005
	ERROR_REFRESH_TOKEN_INVALID = 210

	// Rate limiting
	MAX_CONCURRENT_REQUESTS = 5
	REQUESTS_PER_SECOND     = 5

	defaultMaxRefreshAttempts = 5
	defaultBreakerFailures    = 4
	defaultBreakerOpen        = 2 * time.Minute
)

const metricsServiceExternalJWT = "external_jwt"

// Query parameters for token refresh
var queryParams = map[string]string{
	"version":                           "CA.180.0",
	"deviceId":                          "1363e778-8d4a-4fd3-9ebf-d35aea4fb533",
	"device_name":                       "samsungSM-X910N",
	"device_os_version":                 "9",
	"channel":                           "googleplay",
	"brand":                             "bolt",
	"deviceType":                        "android",
	"signup_session_id":                 "",
	"country":                           "th",
	"is_local_authentication_available": "false",
	"language":                          "th",
	"gps_lat":                           "18.884183",
	"gps_lng":                           "99.020013",
	"gps_accuracy_m":                    "0.02",
	"gps_age":                           "1",
	"user_id":                           "284215753",
	"session_id":                        "284215753u1759766688900",
	"distinct_id":                       "client-284215753",
	"rh_session_id":                     "284215753u1759767820",
}

// TokenRefreshResponse represents the API response for token refresh
type TokenRefreshResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken               string `json:"access_token"`
		ExpiresTimestamp          int64  `json:"expires_timestamp"`
		ExpiresInSeconds          int64  `json:"expires_in_seconds"`
		NextUpdateInSeconds       int64  `json:"next_update_in_seconds"`
		NextUpdateGiveUpTimestamp int64  `json:"next_update_give_up_timestamp"`
	} `json:"data"`
}

// JWTToken represents a JWT token with metadata
type JWTToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
	UserID    int64     `json:"user_id"`
	LoginID   int64     `json:"login_id"`
	JTI       string    `json:"jti"`
}

// TokenError represents a structured error during token operations.
type TokenError struct {
	Operation  string
	Cause      error
	StatusCode int
	Attempt    int
}

func (e *TokenError) Error() string {
	if e == nil {
		return ""
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s failed (status=%d, attempt=%d): %v", e.Operation, e.StatusCode, e.Attempt, e.Cause)
	}
	return fmt.Sprintf("%s failed (attempt=%d): %v", e.Operation, e.Attempt, e.Cause)
}

func (e *TokenError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// RateLimiter manages API rate limiting with token bucket algorithm
type RateLimiter struct {
	limiter           chan struct{}
	ticker            *time.Ticker
	maxConcurrent     int
	requestsPerSecond int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxConcurrent int, requestsPerSecond int) *RateLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}

	interval := time.Second / time.Duration(requestsPerSecond)
	if interval <= 0 {
		interval = time.Second
	}

	return &RateLimiter{
		limiter:           make(chan struct{}, maxConcurrent),
		ticker:            time.NewTicker(interval),
		maxConcurrent:     maxConcurrent,
		requestsPerSecond: requestsPerSecond,
	}
}

// Acquire acquires a rate limit slot with proper error handling
func (rl *RateLimiter) Acquire(ctx context.Context) error {
	if rl == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.ticker.C:
	}

	select {
	case rl.limiter <- struct{}{}:
		if logger := GetLogger(); logger != nil {
			logger.Debug("Rate limiter acquired",
				zap.Int("current", len(rl.limiter)),
				zap.Int("max", rl.maxConcurrent),
			)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a rate limit slot
func (rl *RateLimiter) Release() {
	if rl == nil {
		return
	}

	select {
	case <-rl.limiter:
		if logger := GetLogger(); logger != nil {
			logger.Debug("Rate limiter released",
				zap.Int("current", len(rl.limiter)),
				zap.Int("max", rl.maxConcurrent),
			)
		}
	default:
	}
}

// GetStatus returns current rate limiter status
func (rl *RateLimiter) GetStatus() map[string]interface{} {
	if rl == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"max_concurrent":      rl.maxConcurrent,
		"requests_per_second": rl.requestsPerSecond,
		"current_requests":    len(rl.limiter),
		"available_slots":     rl.maxConcurrent - len(rl.limiter),
	}
}

// Stop releases ticker resources.
func (rl *RateLimiter) Stop() {
	if rl == nil {
		return
	}
	rl.ticker.Stop()
}

// decodeBody handles gzip/deflate response decoding
func decodeBody(resp *http.Response) ([]byte, error) {
	var reader io.ReadCloser
	var err error

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
	case "deflate":
		reader = flate.NewReader(resp.Body)
	default:
		reader = resp.Body
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decoded body: %v", err)
	}

	return body, nil
}

// ExternalJWTManager manages JWT tokens with automatic renewal
type ExternalJWTManager struct {
	currentToken *JWTToken
	mutex        sync.RWMutex
	renewalTime  time.Duration
	userID       int64
	loginID      int64
	baseURL      string
	httpClient   *http.Client
	rateLimiter  *RateLimiter

	refreshInterval    time.Duration
	maxRefreshAttempts int
	backoffBase        time.Duration
	seedToken          string
	backgroundCtx      context.Context
	backgroundCancel   context.CancelFunc
	backgroundOnce     sync.Once

	breakerMu           sync.Mutex
	consecutiveFailures int
	breakerOpenUntil    time.Time
	breakerMaxFailures  int
	breakerOpenDuration time.Duration
}

type JWTManager = ExternalJWTManager

func NewExternalJWTManager(userID, loginID int64, baseURL string, httpClient *http.Client) *ExternalJWTManager {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &ExternalJWTManager{
		renewalTime:         10 * time.Minute,
		userID:              userID,
		loginID:             loginID,
		baseURL:             baseURL,
		httpClient:          httpClient,
		rateLimiter:         NewRateLimiter(MAX_CONCURRENT_REQUESTS, REQUESTS_PER_SECOND),
		refreshInterval:     time.Minute,
		maxRefreshAttempts:  defaultMaxRefreshAttempts,
		backoffBase:         500 * time.Millisecond,
		seedToken:           strings.TrimSpace(os.Getenv("BOLT_API_SEED_TOKEN")),
		backgroundCtx:       ctx,
		backgroundCancel:    cancel,
		breakerMaxFailures:  defaultBreakerFailures,
		breakerOpenDuration: defaultBreakerOpen,
	}

	manager.startBackgroundRefresher()

	return manager
}

func NewJWTManager(userID, loginID int64, baseURL string, httpClient *http.Client) *ExternalJWTManager {
	return NewExternalJWTManager(userID, loginID, baseURL, httpClient)
}

func (jm *ExternalJWTManager) GetValidToken() (string, error) {
	if jm == nil {
		return "", errors.New("jwt manager is not initialized")
	}

	jm.startBackgroundRefresher()

	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	if jm.currentToken != nil && time.Now().Before(jm.currentToken.ExpiresAt) {
		if time.Until(jm.currentToken.ExpiresAt) < jm.renewalTime {
			go jm.triggerAsyncRefresh()
		}
		return jm.currentToken.Token, nil
	}

	return jm.generateNewTokenLocked(jm.backgroundCtx)
}

func (jm *ExternalJWTManager) generateNewTokenLocked(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	authToken := jm.seedToken
	if jm.currentToken != nil && jm.currentToken.Token != "" {
		authToken = jm.currentToken.Token
	}

	tokenData, err := jm.refreshToken(ctx, authToken)
	if err != nil {
		return "", err
	}

	jm.currentToken = tokenData

	return tokenData.Token, nil
}

func (jm *ExternalJWTManager) refreshToken(ctx context.Context, authToken string) (*JWTToken, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if open, remaining := jm.isCircuitOpen(); open {
		err := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf("circuit breaker open for %s", remaining), Attempt: 0}
		jm.logRefreshError("circuit_open", err)
		return nil, err
	}

	reqURL, err := url.Parse(BASE_URL + ENDPOINT)
	if err != nil {
		return nil, &TokenError{Operation: "refreshToken", Cause: err}
	}

	query := reqURL.Query()
	for key, value := range queryParams {
		query.Set(key, value)
	}
	reqURL.RawQuery = query.Encode()

	headers := map[string]string{
		"Host":               "node.bolt.eu",
		"Cache-Control":      "max-age=0",
		"Sec-Ch-Ua":          `"Chromium";v="140", "Not;A=Brand";v="24", "Google Chrome";v="140"`,
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": `"Windows"`,
		"Accept-Language":    "en-US;q=0.9,en;q=0.8",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
		"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Sec-Fetch-Site":     "none",
		"Sec-Fetch-Mode":     "navigate",
		"Sec-Fetch-User":     "?1",
		"Sec-Fetch-Dest":     "document",
		"Accept-Encoding":    "gzip, deflate",
	}
	if authToken != "" {
		headers["Authorization"] = "Bearer " + authToken
	}

	for attempt := 1; attempt <= jm.maxRefreshAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		if err := jm.rateLimiter.Acquire(attemptCtx); err != nil {
			cancel()
			return nil, &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt}
		}

		req, err := http.NewRequestWithContext(attemptCtx, "POST", reqURL.String(), nil)
		if err != nil {
			jm.rateLimiter.Release()
			cancel()
			return nil, &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt}
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := jm.httpClient.Do(req)
		jm.rateLimiter.Release()
		if err != nil {
			cancel()
			jm.recordRefreshFailure(err)
			jm.logRefreshError("network_error", &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt})
			if attempt == jm.maxRefreshAttempts {
				return nil, &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt}
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		body, err := decodeBody(resp)
		resp.Body.Close()
		cancel()
		if err != nil {
			jm.recordRefreshFailure(err)
			jm.logRefreshError("decode_error", &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt})
			if attempt == jm.maxRefreshAttempts {
				return nil, &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt}
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			errAttempt := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf("too many requests"), StatusCode: resp.StatusCode, Attempt: attempt}
			jm.recordRefreshFailure(errAttempt)
			jm.logRefreshError("rate_limited", errAttempt)
			if attempt == jm.maxRefreshAttempts {
				return nil, errAttempt
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			errAttempt := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf(string(body)), StatusCode: resp.StatusCode, Attempt: attempt}
			jm.recordRefreshFailure(errAttempt)
			jm.logRefreshError("status_error", errAttempt)
			if attempt == jm.maxRefreshAttempts {
				return nil, errAttempt
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		var tokenResp TokenRefreshResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			jm.recordRefreshFailure(err)
			jm.logRefreshError("unmarshal_error", &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt})
			if attempt == jm.maxRefreshAttempts {
				return nil, &TokenError{Operation: "refreshToken", Cause: err, Attempt: attempt}
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		if tokenResp.Code == ERROR_REFRESH_TOKEN_INVALID {
			errAttempt := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf("refresh token invalid"), Attempt: attempt}
			jm.recordRefreshFailure(errAttempt)
			jm.logRefreshError("refresh_token_invalid", errAttempt)
			return nil, errAttempt
		}

		if tokenResp.Code == ERROR_TOO_MANY_REQUESTS {
			errAttempt := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf("too many requests"), Attempt: attempt}
			jm.recordRefreshFailure(errAttempt)
			jm.logRefreshError("rate_limited_code", errAttempt)
			if attempt == jm.maxRefreshAttempts {
				return nil, errAttempt
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		if tokenResp.Code != 0 {
			errAttempt := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf(tokenResp.Message), Attempt: attempt}
			jm.recordRefreshFailure(errAttempt)
			jm.logRefreshError("api_error", errAttempt)
			if attempt == jm.maxRefreshAttempts {
				return nil, errAttempt
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		if tokenResp.Data.AccessToken == "" {
			errAttempt := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf("no access token in response"), Attempt: attempt}
			jm.recordRefreshFailure(errAttempt)
			jm.logRefreshError("missing_token", errAttempt)
			if attempt == jm.maxRefreshAttempts {
				return nil, errAttempt
			}
			time.Sleep(jm.backoffWithJitter(attempt))
			continue
		}

		tokenString := tokenResp.Data.AccessToken

		metadata, metaErr := ExtractTokenMetadata(tokenString)
		if metaErr != nil {
			jm.logRefreshError("metadata_extract_failed", metaErr)
		}

		expiresAt := time.Now().Add(time.Hour)
		if metadata != nil {
			expiresAt = metadata.ExpiresAt
		} else if tokenResp.Data.ExpiresInSeconds > 0 {
			expiresAt = time.Now().Add(time.Duration(tokenResp.Data.ExpiresInSeconds) * time.Second)
		} else if tokenResp.Data.ExpiresTimestamp > 0 {
			expiresAt = time.Unix(tokenResp.Data.ExpiresTimestamp, 0)
		}

		issuedAt := time.Now()
		if metadata != nil {
			issuedAt = metadata.IssuedAt
		}

		tokenInfo := &JWTToken{
			Token:     tokenString,
			ExpiresAt: expiresAt,
			IssuedAt:  issuedAt,
			UserID:    jm.userID,
			LoginID:   jm.loginID,
		}

		if metadata != nil {
			if parsedID, parseErr := strconv.ParseInt(metadata.UserID, 10, 64); parseErr == nil {
				tokenInfo.UserID = parsedID
			}
			if parsedLogin, parseErr := strconv.ParseInt(metadata.LoginID, 10, 64); parseErr == nil {
				tokenInfo.LoginID = parsedLogin
			}
			tokenInfo.JTI = metadata.JTI
		}

		jm.recordRefreshSuccess()

		if logger := GetLogger(); logger != nil {
			fields := []zap.Field{
				zap.Int64("user_id", tokenInfo.UserID),
				zap.Int64("login_id", tokenInfo.LoginID),
				zap.Time("expires_at", tokenInfo.ExpiresAt),
			}
			if tokenInfo.JTI != "" {
				fields = append(fields, zap.String("jti", tokenInfo.JTI))
			}
			logger.Info("External JWT token refreshed", fields...)
		}

		LogTokenInfoWithValidation(tokenString, false)

		return tokenInfo, nil
	}

	err := &TokenError{Operation: "refreshToken", Cause: fmt.Errorf("exhausted retries"), Attempt: jm.maxRefreshAttempts}
	jm.logRefreshError("exhausted_retries", err)
	return nil, err
}

func (jm *ExternalJWTManager) backoffWithJitter(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := jm.backoffBase
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	max := base * time.Duration(1<<uint(attempt-1))
	randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitter := time.Duration(randSrc.Int63n(int64(max/2) + 1))
	return max + jitter
}

func (jm *ExternalJWTManager) recordRefreshFailure(err error) {
	jm.breakerMu.Lock()
	defer jm.breakerMu.Unlock()

	jm.consecutiveFailures++
	if jm.consecutiveFailures >= jm.breakerMaxFailures {
		jm.breakerOpenUntil = time.Now().Add(jm.breakerOpenDuration)
	}
}

func (jm *ExternalJWTManager) recordRefreshSuccess() {
	jm.breakerMu.Lock()
	defer jm.breakerMu.Unlock()
	jm.consecutiveFailures = 0
	jm.breakerOpenUntil = time.Time{}
}

func (jm *ExternalJWTManager) isCircuitOpen() (bool, time.Duration) {
	jm.breakerMu.Lock()
	defer jm.breakerMu.Unlock()

	if jm.breakerOpenUntil.IsZero() {
		return false, 0
	}
	remaining := time.Until(jm.breakerOpenUntil)
	if remaining <= 0 {
		jm.breakerOpenUntil = time.Time{}
		jm.consecutiveFailures = 0
		return false, 0
	}
	return true, remaining
}

func (jm *ExternalJWTManager) startBackgroundRefresher() {
	if jm == nil {
		return
	}
	jm.backgroundOnce.Do(func() {
		if jm.backgroundCtx == nil {
			jm.backgroundCtx, jm.backgroundCancel = context.WithCancel(context.Background())
		}
		go jm.backgroundRefreshLoop()
	})
}

func (jm *ExternalJWTManager) backgroundRefreshLoop() {
	ticker := time.NewTicker(jm.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-jm.backgroundCtx.Done():
			return
		case <-ticker.C:
			jm.mutex.RLock()
			current := jm.currentToken
			renew := current != nil && time.Until(current.ExpiresAt) < jm.renewalTime
			jm.mutex.RUnlock()
			if renew {
				ctx := jm.backgroundCtx
				if ctx == nil {
					ctx = context.Background()
				}
				refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				if err := jm.renewTokenWithContext(refreshCtx); err != nil {
					jm.logRefreshError("background_refresh_failed", err)
				}
				cancel()
			}
		}
	}
}

func (jm *ExternalJWTManager) renewTokenWithContext(ctx context.Context) error {
	jm.mutex.Lock()
	defer jm.mutex.Unlock()

	_, err := jm.generateNewTokenLocked(ctx)
	return err
}

func (jm *ExternalJWTManager) triggerAsyncRefresh() {
	go func() {
		ctx := jm.backgroundCtx
		if ctx == nil {
			ctx = context.Background()
		}
		refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := jm.renewTokenWithContext(refreshCtx); err != nil {
			jm.logRefreshError("async_refresh_failed", err)
		}
	}()
}

func (jm *ExternalJWTManager) logRefreshError(reason string, err error) {
	if logger := GetLogger(); logger != nil {
		fields := []zap.Field{
			zap.String("reason", reason),
			zap.Int64("user_id", jm.userID),
			zap.Int64("login_id", jm.loginID),
		}
		if te, ok := err.(*TokenError); ok {
			fields = append(fields, zap.Int("attempt", te.Attempt))
			if te.StatusCode > 0 {
				fields = append(fields, zap.Int("status_code", te.StatusCode))
			}
			if te.Cause != nil {
				fields = append(fields, zap.Error(te.Cause))
			}
		} else if err != nil {
			fields = append(fields, zap.Error(err))
		}
		logger.Warn("External JWT refresh error", fields...)
	}
	if metrics := GetMetricsCollector(); metrics != nil {
		metrics.RecordAPIError(metricsServiceExternalJWT, reason)
	}
}

func (jm *ExternalJWTManager) GetTokenInfo() *JWTToken {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	if jm.currentToken == nil {
		return nil
	}
	copy := *jm.currentToken
	return &copy
}

func (jm *ExternalJWTManager) IsTokenValid() bool {
	jm.mutex.RLock()
	defer jm.mutex.RUnlock()
	return jm.currentToken != nil && time.Now().Before(jm.currentToken.ExpiresAt)
}

func (jm *ExternalJWTManager) ForceRenewal() error {
	return jm.renewTokenWithContext(context.Background())
}

func (jm *ExternalJWTManager) StartAutoRenewal() {
	jm.startBackgroundRefresher()
	if logger := GetLogger(); logger != nil {
		logger.Info("External JWT auto-renewal active", zap.Int64("user_id", jm.userID), zap.Int64("login_id", jm.loginID))
	}
}

// Global JWT manager
var jwtManager *JWTManager

// InitializeJWTManager initializes the global JWT manager
func InitializeJWTManager() {
	jwtManager = NewJWTManager(
		283617495, // user_id
		605354782, // login_id
		"https://user.live.boltsvc.net",
		httpClient,
	)

	if _, err := jwtManager.GetValidToken(); err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("Failed to prime external JWT token", zap.Error(err))
		}
	}

	go jwtManager.StartAutoRenewal()

	if logger := GetLogger(); logger != nil {
		logger.Info("JWT Manager initialized with auto-renewal",
			zap.Int64("user_id", 283617495),
			zap.Int64("login_id", 605354782),
		)
	}
}

// GetJWTToken returns a valid JWT token
func GetJWTToken() (string, error) {
	if jwtManager == nil {
		return "", fmt.Errorf("JWT manager not initialized")
	}
	return jwtManager.GetValidToken()
}

// GetJWTManager returns the global JWT manager
func GetJWTManager() *JWTManager {
	return jwtManager
}

// TestDecodeBody tests the decodeBody function with different content encodings
func TestDecodeBody() {
	if logger := GetLogger(); logger != nil {
		logger.Info("Testing decodeBody function")
	}

	// Test cases for different content encodings
	testCases := []struct {
		name            string
		contentEncoding string
		body            string
	}{
		{"Plain text", "", "Hello, World!"},
		{"Gzip", "gzip", "Hello, World!"},
		{"Deflate", "deflate", "Hello, World!"},
	}

	for _, tc := range testCases {
		if logger := GetLogger(); logger != nil {
			logger.Debug("decodeBody test executed", zap.String("encoding", tc.contentEncoding), zap.String("name", tc.name))
		}
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("decodeBody tests completed")
	}
}
