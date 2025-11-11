package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CSRFManager manages CSRF tokens for the application
type CSRFManager struct {
	tokens map[string]time.Time
	mutex  sync.RWMutex
	secret []byte
}

// NewCSRFManager creates a new CSRF manager
func NewCSRFManager() *CSRFManager {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic("Failed to generate CSRF secret: " + err.Error())
	}

	manager := &CSRFManager{
		tokens: make(map[string]time.Time),
		secret: secret,
	}

	// Start cleanup goroutine
	go manager.cleanupExpiredTokens()

	return manager
}

// GenerateToken generates a new CSRF token
func (m *CSRFManager) GenerateToken() string {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return ""
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	m.mutex.Lock()
	m.tokens[token] = time.Now().Add(24 * time.Hour) // Token expires in 24 hours
	m.mutex.Unlock()

	return token
}

// ValidateToken validates a CSRF token
func (m *CSRFManager) ValidateToken(token string) bool {
	if token == "" {
		return false
	}

	m.mutex.RLock()
	expiry, exists := m.tokens[token]
	m.mutex.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// Token expired, remove it
		m.mutex.Lock()
		delete(m.tokens, token)
		m.mutex.Unlock()
		return false
	}

	return true
}

// ConsumeToken validates and removes a CSRF token (one-time use)
func (m *CSRFManager) ConsumeToken(token string) bool {
	if token == "" {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	expiry, exists := m.tokens[token]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		delete(m.tokens, token)
		return false
	}

	// Remove token after successful validation (one-time use)
	delete(m.tokens, token)
	return true
}

// cleanupExpiredTokens periodically removes expired tokens
func (m *CSRFManager) cleanupExpiredTokens() {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.mutex.Lock()
		for token, expiry := range m.tokens {
			if now.After(expiry) {
				delete(m.tokens, token)
			}
		}
		m.mutex.Unlock()
	}
}

// Middleware returns a Gin middleware for CSRF protection
func (m *CSRFManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for GET, HEAD, OPTIONS requests
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Skip CSRF check for non-admin API calls
		if !isAdminAPIPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Get token from header
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			// Also check form data as fallback
			token = c.PostForm("_csrf_token")
		}

		// Validate token (reusable)
		if !m.ValidateToken(token) {
			c.JSON(403, gin.H{
				"error": "CSRF token invalid or missing",
				"code":  "CSRF_INVALID",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// isAdminAPIPath checks if the path requires CSRF protection
func isAdminAPIPath(path string) bool {
	// Only protect admin API endpoints that modify data
	return path == "/admin/api/endpoints" ||
		path == "/admin/api/endpoints/reorder" ||
		path == "/admin/api/taggers" ||
		path == "/admin/api/logs/cleanup" ||
		path == "/admin/api/config" ||
		path == "/admin/api/settings" ||
		// Pattern for specific endpoint operations
		(len(path) > len("/admin/api/endpoints/") && 
			(path[:len("/admin/api/endpoints/")] == "/admin/api/endpoints/" ||
			 path[:len("/admin/api/taggers/")] == "/admin/api/taggers/"))
}

// SecureCompare performs constant-time string comparison
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}