package router

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func OprecStaff(
	app *gin.Engine,
	handler *handler.OprecStaffHandler,
	authHandler *handler.AuthHandler,
	authService service.AuthService,
	cfg config.Config,
) {
	api := app.Group("/api/v1")
	oprec := api.Group("/oprec-staff")
	oprec.GET("/config", handler.Config)
	oprec.POST("", handler.Register)
	oprec.POST("/submission-status", handler.ConfirmRegistration)

	auth := api.Group("/auth")
	auth.Use(middleware.RequireAllowedOrigin(cfg.HTTP.AllowedOrigins))
	auth.POST("/login", authHandler.Login)
	auth.GET("/me", middleware.RequireAdmin(authService, cfg.Auth.CookieName), authHandler.Me)
	auth.POST("/logout", middleware.RequireAdmin(authService, cfg.Auth.CookieName), authHandler.Logout)

	admin := oprec.Group("")
	admin.Use(
		middleware.RequireAllowedOrigin(cfg.HTTP.AllowedOrigins),
		middleware.RequireAdmin(authService, cfg.Auth.CookieName),
	)
	admin.GET("", handler.List)
	admin.GET("/count", handler.Counts)
	admin.GET("/count/subdivision", handler.PrimarySubdivisionCounts)
	admin.GET("/activity-logs", middleware.RequireSuperAdmin(), handler.ActivityLogs)
	admin.GET("/export/csv", handler.ExportCSV)
	admin.GET("/export/excel", handler.ExportExcel)
	admin.PATCH("/:id/status", handler.UpdateStatus)
	admin.GET("/:id", handler.Detail)
	admin.GET("/:id/documents/:document_id", handler.Document)
}
