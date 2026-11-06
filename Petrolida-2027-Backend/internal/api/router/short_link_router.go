package router

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func ShortLinks(
	app *gin.Engine,
	handler *handler.ShortLinkHandler,
	authService service.AuthService,
	cfg config.Config,
) {
	app.GET("/go/:slug", handler.Redirect)

	routes := app.Group("/api/v1/short-links")
	routes.Use(
		middleware.RequireAllowedOrigin(cfg.HTTP.AllowedOrigins),
		middleware.RequireAdmin(authService, cfg.Auth.CookieName),
	)
	routes.GET("", handler.List)
	routes.POST("", handler.Create)
	routes.DELETE("/:id", handler.Delete)
}
