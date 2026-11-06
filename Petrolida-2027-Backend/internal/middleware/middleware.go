package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Middleware struct {
	db *gorm.DB
	//abuseBanService service.AbuseBanService
}

func New(db ...*gorm.DB) Middleware {
	middleware := Middleware{}
	if len(db) > 0 {
		middleware.db = db[0]
	}

	return middleware
}

func (m Middleware) RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx.Set("request_id", requestID)
		ctx.Header("X-Request-ID", requestID)
		ctx.Next()
	}
}

func (m Middleware) AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startedAt := time.Now()
		ctx.Next()

		log.Info("http request",
			"request_id", ctx.GetString("request_id"),
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"status", ctx.Writer.Status(),
			"latency_ms", time.Since(startedAt).Milliseconds(),
			"client_ip", ctx.ClientIP(),
		)
	}
}

func (m Middleware) Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered",
					"request_id", ctx.GetString("request_id"),
					"error", recovered,
					"stack", string(debug.Stack()),
				)
				res := response.BuildResponseFailed(
					"internal server error",
					myerror.New(myerror.TypeInternal, "INTERNAL_SERVER_ERROR", "internal server error"),
				)
				res.SendWithAbort(ctx)
			}
		}()
		ctx.Next()
	}
}

func (m Middleware) CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		_, explicitlyAllowed := allowed[origin]
		_, wildcardAllowed := allowed["*"]

		if origin != "" && (explicitlyAllowed || wildcardAllowed) {
			// Kredensial hanya boleh dikirim ke origin yang didaftarkan satu per
			// satu. Kalau konfigurasinya "*", memantulkan Origin penelepon sambil
			// menyalakan Allow-Credentials membuat situs mana pun bisa membaca
			// respons admin memakai cookie sesi korban, jadi untuk kasus itu
			// kita balas "*" tanpa kredensial.
			if explicitlyAllowed {
				ctx.Header("Access-Control-Allow-Origin", origin)
				ctx.Header("Access-Control-Allow-Credentials", "true")
			} else {
				ctx.Header("Access-Control-Allow-Origin", "*")
			}
			ctx.Header("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID, X-CSRF-Token")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			ctx.Header("Access-Control-Expose-Headers", "X-Request-ID, Content-Disposition")
			ctx.Header("Vary", "Origin")
		}

		if ctx.Request.Method == http.MethodOptions {
			if origin != "" && !explicitlyAllowed && !wildcardAllowed {
				res := response.BuildResponseFailed(
					"origin is not allowed",
					myerror.New(myerror.TypeAuthorization, "ORIGIN_NOT_ALLOWED", "origin is not allowed"),
				)
				res.SendWithAbort(ctx)
				return
			}

			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

func (m Middleware) BodyLimit(limit int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.Body != nil {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
		}
		ctx.Next()
	}
}

func (m Middleware) Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestContext, cancel := context.WithTimeout(ctx.Request.Context(), timeout)
		defer cancel()

		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Next()

		if requestContext.Err() == context.DeadlineExceeded && !ctx.Writer.Written() {
			res := response.BuildResponseFailed(
				"request timed out",
				myerror.New(myerror.TypeInternal, "REQUEST_TIMEOUT", "request timed out"),
			).SetStatus(http.StatusGatewayTimeout)
			res.SendWithAbort(ctx)
		}
	}
}
