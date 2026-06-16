package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/trae-framework/vine/ctx"
)

func Recovery() ctx.HandlerFunc {
	return func(c *ctx.Ctx) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[VINE] panic recovered: %v", err)
				c.Abort()
				c.HandleError(fmt.Errorf("%v", err))
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
			c.Abort()
			c.Unauthorized("missing Authorization header")
			return
		}

		if !strings.HasPrefix(token, "Bearer ") {
			c.Abort()
			c.Unauthorized("invalid Authorization format, expected 'Bearer <token>'")
			return
		}

		authInfo := parseToken(token)

		if requiredRole != "" && authInfo.Role != requiredRole {
			c.Abort()
			c.Forbidden(fmt.Sprintf("role '%s' required, but got '%s'", requiredRole, authInfo.Role))
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
	payload := strings.TrimSpace(token[7:])

	role := "user"
	if strings.EqualFold(payload, "admin") {
		role = "admin"
	}

	return &AuthInfo{
		UserID:   payload,
		Role:     role,
		Verified: true,
	}
}
