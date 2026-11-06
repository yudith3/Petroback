package handler

import (
	"net/http"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ShortLinkHandler struct {
	service service.ShortLinkService
}

func NewShortLink(shortLinkService service.ShortLinkService) *ShortLinkHandler {
	return &ShortLinkHandler{service: shortLinkService}
}

func (h *ShortLinkHandler) Create(ctx *gin.Context) {
	var input domain.CreateShortLinkInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		response.BuildResponseFailed(
			"invalid short link",
			myerror.NewFieldValidation("body", "Short link", "must be valid JSON"),
		).Send(ctx)
		return
	}

	profile, ok := ctx.Get(middleware.AdminProfileContextKey)
	if !ok {
		response.BuildResponseFailed("authentication required", domain.ErrUnauthenticated).Send(ctx)
		return
	}
	admin, ok := profile.(domain.AdminProfile)
	if !ok {
		response.BuildResponseFailed("authentication required", domain.ErrUnauthenticated).Send(ctx)
		return
	}

	link, err := h.service.Create(input, admin.ID)
	if err != nil {
		response.BuildResponseFailed("failed to create short link", err).Send(ctx)
		return
	}
	response.BuildResponseSuccess("short link created", link).SetStatus(http.StatusCreated).Send(ctx)
}

func (h *ShortLinkHandler) List(ctx *gin.Context) {
	links, err := h.service.List()
	if err != nil {
		response.BuildResponseFailed("failed to get short links", err).Send(ctx)
		return
	}
	response.BuildResponseSuccess("short links retrieved", links).Send(ctx)
}

func (h *ShortLinkHandler) Delete(ctx *gin.Context) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		response.BuildResponseFailed(
			"invalid short link id",
			myerror.NewFieldValidation("id", "Short link", "must be a valid UUID"),
		).Send(ctx)
		return
	}
	if err := h.service.Delete(id); err != nil {
		response.BuildResponseFailed("failed to delete short link", err).Send(ctx)
		return
	}
	response.BuildResponseSuccess("short link deleted", response.EmptyObj{}).Send(ctx)
}

func (h *ShortLinkHandler) Redirect(ctx *gin.Context) {
	destination, err := h.service.Resolve(ctx.Param("slug"))
	if err != nil {
		response.BuildResponseFailed("short link not found", err).Send(ctx)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.Redirect(http.StatusTemporaryRedirect, destination)
}
