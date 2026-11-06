package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var defaultAllowedOrigins = []string{
	"https://petrolida.com",
	"https://www.petrolida.com",
	"http://localhost:3000",
	"http://localhost:3001",
}

func (m Middleware) CORSMiddleware() gin.HandlerFunc {
	allowedOrigins := buildAllowedOrigins()

	return func(ctx *gin.Context) {
		origin := strings.TrimSuffix(ctx.Request.Header.Get("Origin"), "/")
		if _, ok := allowedOrigins[origin]; ok {
			ctx.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			ctx.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			ctx.Writer.Header().Add("Vary", "Origin")
		}

		ctx.Writer.Header().Set("Access-Control-Max-Age", "86400")
		ctx.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PATCH, PUT, DELETE, UPDATE")
		ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, api_key, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		ctx.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Disposition, X-Request-ID")
		ctx.Writer.Header().Set("Cache-Control", "no-cache")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(200)
		} else {
			ctx.Next()
		}
	}
}

func buildAllowedOrigins() map[string]struct{} {
	origins := make(map[string]struct{}, len(defaultAllowedOrigins)+3)

	addOrigins(origins, defaultAllowedOrigins...)
	addOrigins(origins, os.Getenv("FRONTEND_URL"))
	addOrigins(origins, strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")...)

	return origins
}

func addOrigins(origins map[string]struct{}, values ...string) {
	for _, value := range values {
		origin := strings.TrimSpace(strings.TrimSuffix(value, "/"))
		if origin == "" {
			continue
		}

		origins[origin] = struct{}{}
	}
}
