package middleware

import (
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	MyError "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	myjwt "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/jwt"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	MESSAGE_FAILED_VERIFY_TOKEN = "failed to verify token"
	MESSAGE_USER_NOT_AUTHORIZED = "user not authorized"
	MESSAGE_API_IS_LOCKED       = "api is now locked"
)

var (
	ErrTokenInvalid    = MyError.New(MyError.TypeAuthentication, "TOKEN_INVALID", "token invalid")
	ErrTokenNotFound   = MyError.New(MyError.TypeAuthentication, "TOKEN_NOT_FOUND", "token not found")
	ErrTokenExpired    = MyError.New(MyError.TypeAuthentication, "TOKEN_EXPIRED", "token expired")
	ErrRoleNotAllowed  = MyError.New(MyError.TypeAuthorization, "ROLE_NOT_ALLOWED", "role not allowed")
	ErrTokenNotAllowed = MyError.New(MyError.TypeAuthentication, "TOKEN_NOT_ALLOWED", "token not allowed")
)

func (m Middleware) OnlyAllow(roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userRole := ctx.MustGet("role").(string)

		for _, role := range roles {
			if userRole == role {
				ctx.Next()
				return
			}
		}

		res := response.BuildResponseFailed(MESSAGE_USER_NOT_AUTHORIZED, ErrRoleNotAllowed)
		res.SendWithAbort(ctx)
	}
}

func (m Middleware) Authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			res := response.BuildResponseFailed(MESSAGE_FAILED_VERIFY_TOKEN, ErrTokenNotFound)
			res.SendWithAbort(ctx)
			return
		}

		if !strings.Contains(authHeader, "Bearer ") {
			res := response.BuildResponseFailed(MESSAGE_FAILED_VERIFY_TOKEN, ErrTokenInvalid)
			res.SendWithAbort(ctx)
			return
		}

		authHeader = strings.Replace(authHeader, "Bearer ", "", -1)

		idToken, err := myjwt.GetPayloadInsideToken(authHeader)
		if err != nil {
			if err.Error() == "token expired" {
				res := response.BuildResponseFailed(MESSAGE_FAILED_VERIFY_TOKEN, ErrTokenExpired)
				res.SendWithAbort(ctx)
				return
			}

			res := response.BuildResponseFailed(MESSAGE_FAILED_VERIFY_TOKEN, domain.ErrGeneral)
			res.SendWithAbort(ctx)
			return
		}

		ctx.Set("token", authHeader)
		ctx.Set("payload", idToken)
		ctx.Set("user_id", idToken["user_id"])
		ctx.Set("email", idToken["email"])
		ctx.Set("role", idToken["role"])
		ctx.Next()
	}
}
