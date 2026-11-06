package middleware

import (
	"net/http"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const AdminProfileContextKey = "admin_profile"

type AdminAuthenticator interface {
	Authenticate(token string) (domain.AdminProfile, error)
}

func RequireAdmin(auth AdminAuthenticator, cookieName string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, _ := ctx.Cookie(cookieName)
		if token == "" {
			header := strings.TrimSpace(ctx.GetHeader("Authorization"))
			if strings.HasPrefix(header, "Bearer ") {
				token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			}
		}
		if token == "" {
			response.BuildResponseFailed("authentication required", domain.ErrUnauthenticated).SendWithAbort(ctx)
			return
		}
		profile, err := auth.Authenticate(token)
		if err != nil {
			response.BuildResponseFailed("authentication required", err).SendWithAbort(ctx)
			return
		}
		ctx.Set(AdminProfileContextKey, profile)
		ctx.Next()
	}
}

func RequireSuperAdmin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		value, ok := ctx.Get(AdminProfileContextKey)
		profile, valid := value.(domain.AdminProfile)
		if !ok || !valid || !profile.IsSuperAdmin() {
			response.BuildResponseFailed("superadmin access is required", domain.ErrForbidden).SendWithAbort(ctx)
			return
		}
		ctx.Next()
	}
}

func RequireAllowedOrigin(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSuffix(origin, "/")] = struct{}{}
	}
	return func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodGet || ctx.Request.Method == http.MethodHead || ctx.Request.Method == http.MethodOptions {
			ctx.Next()
			return
		}
		origin := strings.TrimSuffix(ctx.GetHeader("Origin"), "/")
		if origin != "" {
			if _, ok := allowed[origin]; !ok {
				response.BuildResponseFailed("origin is not allowed", domain.ErrForbidden).SendWithAbort(ctx)
				return
			}
		}
		ctx.Next()
	}
}
