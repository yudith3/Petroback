package router

import (
	"log/slog"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func New(cfg config.Config, database handler.DatabasePinger, log *slog.Logger) *gin.Engine {
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	middleware := middleware.New()
	engine := gin.New()
	_ = engine.SetTrustedProxies(nil)
	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.AccessLog(log),
		middleware.CORS(cfg.HTTP.AllowedOrigins),
		middleware.BodyLimit(cfg.HTTP.RequestBodyLimit),
		middleware.Timeout(cfg.HTTP.WriteTimeout),
	)

	healthHandler := handler.NewHealth(database, cfg.App.Name, cfg.App.Environment)
	api := engine.Group("/api/v1")
	api.GET("/health", healthHandler.Check)

	engine.NoRoute(func(ctx *gin.Context) {
		response.BuildResponseFailed("route not found", domain.ErrRouteNotFound).Send(ctx)
	})

	return engine
}
