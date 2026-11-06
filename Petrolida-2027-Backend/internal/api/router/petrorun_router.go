package router

import (
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Batas cek tiket dan verifikasi QR per IP. Ticket number dan token sudah
// sulit ditebak, limiter ini lapisan kedua terhadap tebak massal.
const (
	petroRunTicketRateLimit  = 30
	petroRunTicketRateWindow = time.Minute
)

func PetroRun(app *gin.Engine, h *handler.PetroRunHandler, authService service.AuthService, cfg config.Config) {
	// Satu limiter dipakai dua route, jadi kuotanya dibagi.
	ticketLimiter := middleware.New().RateLimit(petroRunTicketRateLimit, petroRunTicketRateWindow)

	routes := app.Group("/api/v1/petrorun")
	{
		routes.GET("/config", h.Config)
		routes.POST("/orders", h.CreateOrder)
		routes.GET("/orders/:order_id", h.GetOrder)
		routes.GET("/orders/:order_id/tickets", h.ListTickets)
		routes.POST("/orders/:order_id/payment-proofs", h.SubmitPaymentProof)
		routes.GET("/tickets/:ticket_number", ticketLimiter, h.LookupTicket)
		routes.POST("/tickets/verify", ticketLimiter, h.VerifyTicket)
	}
	admin := routes.Group("/admin")
	admin.Use(
		middleware.RequireAllowedOrigin(cfg.HTTP.AllowedOrigins),
		middleware.RequireAdmin(authService, cfg.Auth.CookieName),
		middleware.RequireSuperAdmin(),
	)
	proofs := admin.Group("/payment-proofs")
	proofs.GET("", h.ListPaymentProofs)
	proofs.GET("/:proof_id/file", h.GetPaymentProofFile)
	proofs.POST("/:proof_id/review", h.ReviewPaymentProof)
	orders := admin.Group("/orders")
	orders.GET("", h.ListAdminOrders)
	orders.GET("/summary", h.SummarizeAdminOrders)
	orders.GET("/:order_id", h.GetAdminOrder)
}
