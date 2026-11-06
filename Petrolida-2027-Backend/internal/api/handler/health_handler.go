package handler

import (
	"context"
	"net/http"
	"time"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	pagination "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/meta"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type DatabasePinger interface {
	PingContext(context.Context) error
}

const healthDatabaseTimeout = 8 * time.Second

type HealthHandler struct {
	database    DatabasePinger
	serviceName string
	environment string
}

type HealthData struct {
	Status      string    `json:"status"`
	Service     string    `json:"service"`
	Environment string    `json:"environment"`
	Timestamp   time.Time `json:"timestamp"`
}

func NewHealth(database DatabasePinger, serviceName, environment string) *HealthHandler {
	return &HealthHandler{
		database:    database,
		serviceName: serviceName,
		environment: environment,
	}
}

func (h *HealthHandler) Check(ctx *gin.Context) {
	// The production VPS reaches PostgreSQL through Supabase's regional pooler.
	// Establishing a connection can take several seconds after an idle period,
	// so a two-second deadline produced false 503 responses for a healthy DB.
	checkContext, cancel := context.WithTimeout(ctx.Request.Context(), healthDatabaseTimeout)
	defer cancel()

	if err := h.database.PingContext(checkContext); err != nil {
		res := response.BuildResponseFailed(
			"database unavailable",
			myerror.New(myerror.TypeInternal, "DATABASE_UNAVAILABLE", err.Error()),
		).SetStatus(http.StatusServiceUnavailable)
		res.Send(ctx)
		return
	}

	res := response.BuildResponseSuccess("Success", HealthData{
		Status:      "ok",
		Service:     h.serviceName,
		Environment: h.environment,
		Timestamp:   time.Now().UTC(),
	}, pagination.Meta{})
	res.Send(ctx)
}
