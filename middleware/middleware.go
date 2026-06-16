package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/trae-framework/vine/ctx"
)

func Recovery() ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[VINE] panic recovered: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error":  "Internal Server Error",
					"detail": fmt.Sprintf("%v", err),
				})
			}
		}()
		c.Next()
	}
}

func Logger() ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		start := time.Now()
		path := c.Path()
		method := c.Method()

		log.Printf("[VINE] --> %s %s", method, path)

		c.Next()

		latency := time.Since(start)
		statusCode := c.StatusCode()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		log.Printf("[VINE] <-- %s %s %d %v", method, path, statusCode, latency)
	}
}

func CORS(allowOrigins string) ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		c.Header("Access-Control-Allow-Origin", allowOrigins)
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Method() == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

type TimingRecord struct {
	Name     string
	Duration time.Duration
}

func Timing() ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		start := time.Now()

		c.Set("_timing_start", start)

		c.Next()

		total := time.Since(start)
		c.Header("X-Response-Time", total.String())
	}
}

func RequestID() ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		reqID := c.Request.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

type AuthInfo struct {
	UserID   string
	Role     string
	Verified bool
}

func Auth(requiredRole string) ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		token := c.Request.Header.Get("Authorization")
		if token == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "authorization header required",
			})
			return
		}

		if !isValidToken(token) {
			c.AbortWithStatus(http.StatusForbidden)
			c.JSON(http.StatusForbidden, map[string]string{
				"error": "invalid or expired token",
			})
			return
		}

		authInfo := parseToken(token)
		if requiredRole != "" && authInfo.Role != requiredRole {
			c.AbortWithStatus(http.StatusForbidden)
			c.JSON(http.StatusForbidden, map[string]string{
				"error": "insufficient permissions",
			})
			return
		}

		c.Set("auth", authInfo)
		c.Next()
	}
}

func isValidToken(token string) bool {
	return len(token) > 7 && token[:7] == "Bearer "
}

func parseToken(token string) *AuthInfo {
	payload := token[7:]
	return &AuthInfo{
		UserID:   payload,
		Role:     "user",
		Verified: true,
	}
}
