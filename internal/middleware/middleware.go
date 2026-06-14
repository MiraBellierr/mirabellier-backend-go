package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// VaryUserAgent sets Vary: User-Agent for SEO routes where content differs for crawlers vs humans.
func VaryUserAgent(paths ...string) gin.HandlerFunc {
	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}
	return func(c *gin.Context) {
		if pathSet[c.Request.URL.Path] {
			c.Header("Vary", "User-Agent")
		}
		c.Next()
	}
}

// CORS returns a cross-origin middleware for the given allowed origins.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool)
	for _, o := range allowedOrigins {
		o = strings.TrimRight(o, "/")
		allowed[o] = true
	}

	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")

		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		c.Header("Access-Control-Allow-Methods", "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Like-Client-Id,X-Like-Anonymous-Id")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// ServerTiming adds a Server-Timing header with total request duration.
func ServerTiming() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		c.Header("Server-Timing", fmt.Sprintf("total;dur=%.2f", duration.Seconds()/1000))
	}
}

// KeepAlive sets Connection: keep-alive header.
func KeepAlive() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Connection", "keep-alive")
		c.Header("Keep-Alive", "timeout=5, max=100")
		c.Next()
	}
}
