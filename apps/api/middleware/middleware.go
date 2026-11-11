// Package middleware contains the HTTP middleware of the REST API:
// authentication, role checks, rate limiting, request-size limits, CORS and
// security headers.
package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thanhmaitran/compatlab/internal/auth"
	"github.com/thanhmaitran/compatlab/internal/service"
)

// ActorKey is the context key under which the authenticated actor is stored.
const ActorKey = "actor"

// Auth requires a valid bearer token and stores the actor in the context.
func Auth(tokens *auth.TokenIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		claims, err := tokens.Verify(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(ActorKey, service.Actor{UserID: claims.Sub, OrgID: claims.Org, Role: claims.Role})
		c.Next()
	}
}

// RequestSize rejects bodies larger than max bytes.
func RequestSize(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > max {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// RateLimit allows at most limit requests per client IP per window (fixed
// window). A limit of 0 disables it.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	hits := map[string]*entry{}
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		now := time.Now()
		ip := c.ClientIP()
		mu.Lock()
		e := hits[ip]
		if e == nil || now.After(e.reset) {
			if len(hits) > 10000 {
				hits = map[string]*entry{}
			}
			e = &entry{reset: now.Add(window)}
			hits[ip] = e
		}
		e.count++
		over := e.count > limit
		retry := int(time.Until(e.reset).Seconds()) + 1
		mu.Unlock()
		if over {
			c.Header("Retry-After", strconv.Itoa(retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// SecurityHeaders sets conservative response headers.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// CORS allows exactly one configured origin (empty disables CORS headers).
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin != "" && c.GetHeader("Origin") == origin {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}
