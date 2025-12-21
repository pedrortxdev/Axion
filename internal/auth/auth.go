package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// ERROR TAXONOMY
// ============================================================================

type AuthErrorCode int

const (
	ErrCodeTokenMissing AuthErrorCode = iota + 6000
	ErrCodeTokenInvalid
	ErrCodeTokenExpired
	ErrCodeTokenRevoked
	ErrCodeInvalidCredentials
	ErrCodeInvalidSigningMethod
	ErrCodePasswordTooWeak
	ErrCodeRateLimitExceeded
	ErrCodeSecretNotConfigured
	ErrCodeClaimsMissing
)

type AuthError struct {
	Code      AuthErrorCode
	Message   string
	Err       error
	Timestamp int64
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func NewAuthError(code AuthErrorCode, msg string, err error) *AuthError {
	return &AuthError{
		Code:      code,
		Message:   msg,
		Err:       err,
		Timestamp: time.Now().UnixNano(),
	}
}

// Error constructors
func ErrTokenMissing() *AuthError {
	return NewAuthError(ErrCodeTokenMissing, "authentication token required", nil)
}

func ErrTokenInvalid(err error) *AuthError {
	return NewAuthError(ErrCodeTokenInvalid, "invalid token", err)
}

func ErrTokenExpired() *AuthError {
	return NewAuthError(ErrCodeTokenExpired, "token expired", nil)
}

func ErrInvalidCredentials() *AuthError {
	return NewAuthError(ErrCodeInvalidCredentials, "invalid credentials", nil)
}

func ErrRateLimited() *AuthError {
	return NewAuthError(ErrCodeRateLimitExceeded, "too many attempts", nil)
}

// ============================================================================
// CONFIGURATION
// ============================================================================

const (
	defaultTokenDuration  = 24 * time.Hour
	refreshTokenDuration  = 7 * 24 * time.Hour
	maxLoginAttempts      = 5
	rateLimitWindow       = 15 * time.Minute
	bcryptCost            = 12
	minPasswordLength     = 8
	tokenIssuer           = "axion-control-plane"
)

type Config struct {
	SecretKey         []byte
	TokenDuration     time.Duration
	RefreshDuration   time.Duration
	EnableRateLimit   bool
	MaxLoginAttempts  int
	RateLimitWindow   time.Duration
	RequireStrongPass bool
}

func DefaultConfig() *Config {
	secret := getSecret()
	if len(secret) < 32 {
		log.Printf("[SECURITY WARNING] JWT secret is weak (length: %d). Use at least 32 characters in production!", len(secret))
	}

	return &Config{
		SecretKey:         []byte(secret),
		TokenDuration:     defaultTokenDuration,
		RefreshDuration:   refreshTokenDuration,
		EnableRateLimit:   true,
		MaxLoginAttempts:  maxLoginAttempts,
		RateLimitWindow:   rateLimitWindow,
		RequireStrongPass: getEnv("REQUIRE_STRONG_PASSWORD", "true") == "true",
	}
}

func getSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Println("[SECURITY WARNING] JWT_SECRET not set! Using insecure fallback. Set JWT_SECRET environment variable in production!")
		return "axion-insecure-secret-key-change-me"
	}
	return secret
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// ============================================================================
// METRICS
// ============================================================================

type AuthMetrics struct {
	loginAttempts      atomic.Uint64
	loginSuccesses     atomic.Uint64
	loginFailures      atomic.Uint64
	tokenValidations   atomic.Uint64
	tokenRejections    atomic.Uint64
	rateLimitHits      atomic.Uint64
	tokensRevoked      atomic.Uint64
	refreshTokensUsed  atomic.Uint64
}

var globalAuthMetrics = &AuthMetrics{}

func (m *AuthMetrics) Snapshot() map[string]interface{} {
	return map[string]interface{}{
		"login_attempts":      m.loginAttempts.Load(),
		"login_successes":     m.loginSuccesses.Load(),
		"login_failures":      m.loginFailures.Load(),
		"token_validations":   m.tokenValidations.Load(),
		"token_rejections":    m.tokenRejections.Load(),
		"rate_limit_hits":     m.rateLimitHits.Load(),
		"tokens_revoked":      m.tokensRevoked.Load(),
		"refresh_tokens_used": m.refreshTokensUsed.Load(),
	}
}

// ============================================================================
// JWT CLAIMS
// ============================================================================

type AxionClaims struct {
	UserID      string   `json:"uid"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	TokenType   string   `json:"token_type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

func (c *AxionClaims) Validate() error {
	if c.UserID == "" {
		return NewAuthError(ErrCodeClaimsMissing, "user_id missing", nil)
	}
	if c.Role == "" {
		return NewAuthError(ErrCodeClaimsMissing, "role missing", nil)
	}
	if c.TokenType != "access" && c.TokenType != "refresh" {
		return NewAuthError(ErrCodeTokenInvalid, "invalid token type", nil)
	}
	return nil
}

// ============================================================================
// TOKEN REVOCATION (IN-MEMORY)
// ============================================================================

type TokenRevocationStore struct {
	revoked map[string]time.Time // token_id -> expiration
	mu      sync.RWMutex
}

var revokedTokens = &TokenRevocationStore{
	revoked: make(map[string]time.Time),
}

func (s *TokenRevocationStore) Revoke(tokenID string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[tokenID] = expiresAt
	globalAuthMetrics.tokensRevoked.Add(1)
}

func (s *TokenRevocationStore) IsRevoked(tokenID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expiry, exists := s.revoked[tokenID]
	if !exists {
		return false
	}
	// Clean up expired entries
	if time.Now().After(expiry) {
		s.mu.RUnlock()
		s.mu.Lock()
		delete(s.revoked, tokenID)
		s.mu.Unlock()
		s.mu.RLock()
		return false
	}
	return true
}

func (s *TokenRevocationStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	for tokenID, expiry := range s.revoked {
		if now.After(expiry) {
			delete(s.revoked, tokenID)
		}
	}
}

// Periodic cleanup
func StartRevocationCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			revokedTokens.Cleanup()
		case <-ctx.Done():
			return
		}
	}
}

// ============================================================================
// RATE LIMITING
// ============================================================================

type RateLimiter struct {
	attempts map[string][]time.Time // IP -> timestamps
	mu       sync.RWMutex
	config   *Config
}

var rateLimiter = &RateLimiter{
	attempts: make(map[string][]time.Time),
}

func (r *RateLimiter) SetConfig(cfg *Config) {
	r.config = cfg
}

func (r *RateLimiter) CheckLimit(ip string) bool {
	if r.config == nil || !r.config.EnableRateLimit {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.config.RateLimitWindow)

	// Clean old attempts
	if attempts, exists := r.attempts[ip]; exists {
		valid := []time.Time{}
		for _, t := range attempts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		r.attempts[ip] = valid

		if len(valid) >= r.config.MaxLoginAttempts {
			globalAuthMetrics.rateLimitHits.Add(1)
			return false
		}
	}

	// Record attempt
	r.attempts[ip] = append(r.attempts[ip], now)
	return true
}

func (r *RateLimiter) Reset(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, ip)
}

func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.config.RateLimitWindow)

	for ip, attempts := range r.attempts {
		valid := []time.Time{}
		for _, t := range attempts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) > 0 {
			r.attempts[ip] = valid
		} else {
			delete(r.attempts, ip)
		}
	}
}

func StartRateLimitCleanup(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rateLimiter.Cleanup()
		case <-ctx.Done():
			return
		}
	}
}

// ============================================================================
// AUTH SERVICE
// ============================================================================

type AuthService struct {
	config *Config
}

var globalAuthService *AuthService
var serviceOnce sync.Once

func InitAuthService(cfg *Config) *AuthService {
	serviceOnce.Do(func() {
		if cfg == nil {
			cfg = DefaultConfig()
		}
		globalAuthService = &AuthService{config: cfg}
		rateLimiter.SetConfig(cfg)
		
		log.Printf("[Auth] Service initialized (token_duration=%v, rate_limit=%v)",
			cfg.TokenDuration, cfg.EnableRateLimit)
	})
	return globalAuthService
}

func GetAuthService() *AuthService {
	if globalAuthService == nil {
		return InitAuthService(nil)
	}
	return globalAuthService
}

func (s *AuthService) GenerateAccessToken(userID, username, role string, permissions []string) (string, error) {
	tokenID := generateTokenID()
	
	claims := AxionClaims{
		UserID:      userID,
		Username:    username,
		Role:        role,
		Permissions: permissions,
		TokenType:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.TokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    tokenIssuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.config.SecretKey)
}

func (s *AuthService) GenerateRefreshToken(userID, username string) (string, error) {
	tokenID := generateTokenID()
	
	claims := AxionClaims{
		UserID:    userID,
		Username:  username,
		Role:      "refresh",
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.RefreshDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    tokenIssuer,
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.config.SecretKey)
}

func (s *AuthService) ValidateToken(tokenString string) (*AxionClaims, error) {
	globalAuthMetrics.tokenValidations.Add(1)

	token, err := jwt.ParseWithClaims(tokenString, &AxionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, NewAuthError(ErrCodeInvalidSigningMethod, 
				fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]), nil)
		}
		return s.config.SecretKey, nil
	})

	if err != nil {
		globalAuthMetrics.tokenRejections.Add(1)
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired()
		}
		return nil, ErrTokenInvalid(err)
	}

	claims, ok := token.Claims.(*AxionClaims)
	if !ok || !token.Valid {
		globalAuthMetrics.tokenRejections.Add(1)
		return nil, ErrTokenInvalid(errors.New("invalid claims"))
	}

	// Validate custom claims
	if err := claims.Validate(); err != nil {
		globalAuthMetrics.tokenRejections.Add(1)
		return nil, err
	}

	// Check if token is revoked
	if revokedTokens.IsRevoked(claims.ID) {
		globalAuthMetrics.tokenRejections.Add(1)
		return nil, NewAuthError(ErrCodeTokenRevoked, "token has been revoked", nil)
	}

	return claims, nil
}

func (s *AuthService) RevokeToken(tokenString string) error {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return err
	}

	revokedTokens.Revoke(claims.ID, claims.ExpiresAt.Time)
	return nil
}

func (s *AuthService) RefreshAccessToken(refreshTokenString string) (string, error) {
	claims, err := s.ValidateToken(refreshTokenString)
	if err != nil {
		return "", err
	}

	if claims.TokenType != "refresh" {
		return "", NewAuthError(ErrCodeTokenInvalid, "not a refresh token", nil)
	}

	globalAuthMetrics.refreshTokensUsed.Add(1)

	// Generate new access token
	return s.GenerateAccessToken(claims.UserID, claims.Username, "admin", nil)
}

// ============================================================================
// PASSWORD UTILITIES
// ============================================================================

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func ValidatePasswordStrength(password string) error {
	if len(password) < minPasswordLength {
		return NewAuthError(ErrCodePasswordTooWeak, 
			fmt.Sprintf("password must be at least %d characters", minPasswordLength), nil)
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return NewAuthError(ErrCodePasswordTooWeak,
			"password must contain uppercase, lowercase, digit, and special character", nil)
	}

	return nil
}

func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

func AuthMiddleware() gin.HandlerFunc {
	service := GetAuthService()
	
	return func(c *gin.Context) {
		var tokenString string

		// 1. Check query parameter (for WebSocket/Download)
		tokenString = c.Query("token")

		// 2. Check Authorization header
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					tokenString = parts[1]
				}
			}
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"error": "authentication required",
				"code":  ErrCodeTokenMissing,
			})
			return
		}

		claims, err := service.ValidateToken(tokenString)
		if err != nil {
			authErr, ok := err.(*AuthError)
			if ok {
				c.AbortWithStatusJSON(401, gin.H{
					"error": authErr.Message,
					"code":  authErr.Code,
				})
			} else {
				c.AbortWithStatusJSON(401, gin.H{
					"error": "invalid token",
					"code":  ErrCodeTokenInvalid,
				})
			}
			return
		}

		// Set claims in context for use in handlers
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("permissions", claims.Permissions)

		c.Next()
	}
}

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(403, gin.H{"error": "role not found"})
			return
		}

		roleStr := role.(string)
		for _, allowed := range allowedRoles {
			if roleStr == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(403, gin.H{"error": "insufficient permissions"})
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatusJSON(403, gin.H{"error": "permissions not found"})
			return
		}

		permissions := perms.([]string)
		for _, p := range permissions {
			if p == permission {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(403, gin.H{"error": "permission denied"})
	}
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

func LoginHandler(c *gin.Context) {
	service := GetAuthService()
	globalAuthMetrics.loginAttempts.Add(1)

	// Rate limiting
	clientIP := c.ClientIP()
	if !rateLimiter.CheckLimit(clientIP) {
		globalAuthMetrics.loginFailures.Add(1)
		c.JSON(429, gin.H{
			"error": "too many login attempts",
			"code":  ErrCodeRateLimitExceeded,
			"retry_after": int(rateLimitWindow.Seconds()),
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		globalAuthMetrics.loginFailures.Add(1)
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	// Get expected password (in production, check against database with hashed password)
	expectedPass := os.Getenv("AXION_PASSWORD")
	if expectedPass == "" {
		log.Println("[SECURITY WARNING] AXION_PASSWORD not set! Using insecure fallback")
		expectedPass = "admin123"
	}

	// Use constant-time comparison
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(expectedPass)) != 1 {
		globalAuthMetrics.loginFailures.Add(1)
		// Add delay to prevent timing attacks
		time.Sleep(time.Duration(100+rand.Int()%200) * time.Millisecond)
		c.JSON(401, gin.H{
			"error": "invalid credentials",
			"code":  ErrCodeInvalidCredentials,
		})
		return
	}

	// Success - reset rate limiter
	rateLimiter.Reset(clientIP)
	globalAuthMetrics.loginSuccesses.Add(1)

	// Generate tokens
	username := req.Username
	if username == "" {
		username = "admin"
	}

	accessToken, err := service.GenerateAccessToken("admin-001", username, "admin", []string{"*"})
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	refreshToken, err := service.GenerateRefreshToken("admin-001", username)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate refresh token"})
		return
	}

	c.JSON(200, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(service.config.TokenDuration.Seconds()),
	})
}

func RefreshTokenHandler(c *gin.Context) {
	service := GetAuthService()

	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	newAccessToken, err := service.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		authErr, ok := err.(*AuthError)
		if ok {
			c.JSON(401, gin.H{
				"error": authErr.Message,
				"code":  authErr.Code,
			})
		} else {
			c.JSON(401, gin.H{"error": "invalid refresh token"})
		}
		return
	}

	c.JSON(200, gin.H{
		"access_token": newAccessToken,
		"token_type":   "Bearer",
		"expires_in":   int(service.config.TokenDuration.Seconds()),
	})
}

func RevokeTokenHandler(c *gin.Context) {
	service := GetAuthService()

	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	if err := service.RevokeToken(req.Token); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "token revoked"})
}

func GetAuthMetricsHandler(c *gin.Context) {
	c.JSON(200, globalAuthMetrics.Snapshot())
}

// ============================================================================
// INITIALIZATION & SHUTDOWN
// ============================================================================

func Init(cfg *Config) {
	service := InitAuthService(cfg)
	log.Printf("[Auth] Initialized with token duration: %v", service.config.TokenDuration)
}

func StartBackgroundServices(ctx context.Context) {
	go StartRevocationCleanup(ctx)
	go StartRateLimitCleanup(ctx)
	log.Println("[Auth] Background services started")
}

func Shutdown(ctx context.Context) error {
	log.Println("[Auth] Shutting down...")
	// Cleanup can be added here if needed
	return nil
}

// Compatibility functions for existing code
func GenerateToken() (string, error) {
	service := GetAuthService()
	return service.GenerateAccessToken("admin-001", "admin", "admin", []string{"*"})
}

func ValidateToken(tokenString string) (*AxionClaims, error) {
	service := GetAuthService()
	return service.ValidateToken(tokenString)
}