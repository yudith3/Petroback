package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.AuthService
	config  config.AuthConfig
}

func NewAuth(authService service.AuthService, cfg config.AuthConfig) *AuthHandler {
	return &AuthHandler{service: authService, config: cfg}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var request loginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Email) == "" || request.Password == "" {
		response.BuildResponseFailed("invalid login request", domain.ErrInvalidCredentials).Send(ctx)
		return
	}
	token, profile, err := h.service.Login(request.Email, request.Password)
	if err != nil {
		response.BuildResponseFailed("login failed", err).Send(ctx)
		return
	}
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		h.config.CookieName,
		token,
		int(h.config.SessionTTL/time.Second),
		"/",
		"",
		h.config.CookieSecure,
		true,
	)
	response.BuildResponseSuccess("login successful", profile).Send(ctx)
}

func (h *AuthHandler) Me(ctx *gin.Context) {
	profile, ok := ctx.Get(middleware.AdminProfileContextKey)
	if !ok {
		response.BuildResponseFailed("authentication required", domain.ErrUnauthenticated).Send(ctx)
		return
	}
	response.BuildResponseSuccess("admin session is valid", profile).Send(ctx)
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(h.config.CookieName, "", -1, "/", "", h.config.CookieSecure, true)
	response.BuildResponseSuccess("logout successful", response.EmptyObj{}).Send(ctx)
}
