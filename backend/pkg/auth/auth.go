// Package auth provides optional token authentication and a simple per-IP
// rate limiter for the HTTP API.
//
// When the SONEPH_TOKEN environment variable is set, every /api request must
// present the token via one of:
//   - Authorization: Bearer <token>
//   - X-Auth-Token: <token>
//   - ?token=<token> (used by the WebSocket handshake and <audio> streams)
//
// When SONEPH_TOKEN is empty, auth is disabled (local development).
package auth

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// token returns the configured API token. Empty means auth is disabled.
func token() string {
	return os.Getenv("SONEPH_TOKEN")
}

// TokenEnabled reports whether an API token is configured.
func TokenEnabled() bool {
	return token() != ""
}

// extractToken looks for the token in the Authorization header, the
// X-Auth-Token header, or the ?token= query parameter.
func extractToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	if t := c.GetHeader("X-Auth-Token"); t != "" {
		return t
	}
	return c.Query("token")
}

// RequireToken rejects requests without a valid token when one is configured.
func RequireToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !TokenEnabled() {
			c.Next()
			return
		}
		got := extractToken(c)
		want := token()
		if got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing or invalid API token"})
	}
}

// RateLimit is a per-IP sliding-window limiter. It applies whether or not a
// token is configured — a public VPS without a token still gets basic
// protection against hammering.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		count   int
		resetAt time.Time
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := buckets[ip]
		if !ok || now.After(b.resetAt) {
			b = &bucket{count: 0, resetAt: now.Add(window)}
			buckets[ip] = b
		}
		b.count++
		count := b.count
		mu.Unlock()

		if count > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded — slow down"})
			return
		}
		c.Next()
	}
}
