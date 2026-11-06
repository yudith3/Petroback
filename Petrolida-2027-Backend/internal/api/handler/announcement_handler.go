package handler

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"time"
)

type AnnouncementHandler struct{ service service.AnnouncementService }

func NewAnnouncement(s service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{s}
}
func (h *AnnouncementHandler) Config(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	response.BuildResponseSuccess("announcement configuration retrieved", h.service.Config(time.Now())).Send(ctx)
}
func (h *AnnouncementHandler) Lookup(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	var input domain.AnnouncementInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		response.BuildResponseFailed("invalid announcement lookup", myerror.NewFieldValidation("body", "Lookup", "must be valid JSON")).Send(ctx)
		return
	}
	result, err := h.service.Lookup(ctx.Request.Context(), ctx.Param("stage"), input, time.Now())
	if err != nil {
		response.BuildResponseFailed("announcement unavailable", err).Send(ctx)
		return
	}
	response.BuildResponseSuccess("announcement retrieved", result).Send(ctx)
}
