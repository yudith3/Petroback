package handler

import (
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	pagination "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/meta"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *PetroRunHandler) GetOrder(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	token, ok := strings.CutPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		sendPetroRunError(ctx, "failed to get order", domain.ErrPetroRunInvalidAccess)
		return
	}
	result, err := h.service.GetOrder(ctx.Request.Context(), ctx.Param("order_id"), strings.TrimSpace(token))
	if err != nil {
		sendPetroRunError(ctx, "failed to get order", err)
		return
	}
	response.BuildResponseSuccess("order retrieved", result).Send(ctx)
}

func parsePetroRunAdminQuery(ctx *gin.Context) (domain.PetroRunAdminQuery, error) {
	q := domain.PetroRunAdminQuery{Search: strings.TrimSpace(ctx.Query("search")), Status: ctx.Query("status"), Sort: ctx.DefaultQuery("sort", "created_at"), Direction: ctx.DefaultQuery("direction", "desc")}
	page, size, err := parsePagination(ctx, 20, "page_size", "take")
	if err != nil {
		return q, err
	}
	q.Page = page
	q.PageSize = size
	invalid := func(field, message string) error { return myerror.NewFieldValidation(field, field, message) }
	if len(q.Search) > 200 {
		return q, invalid("search", "must not exceed 200 characters")
	}
	switch q.Status {
	case "", "waiting_payment", "pending_review", "approved", "rejected", "expired":
	default:
		return q, invalid("status", "is invalid")
	}
	switch q.Sort {
	case "created_at", "expires_at", "payable_total", "status":
	default:
		return q, invalid("sort", "is invalid")
	}
	if q.Direction != "asc" && q.Direction != "desc" {
		return q, invalid("direction", "must be asc or desc")
	}
	location, _ := time.LoadLocation("Asia/Jakarta")
	for _, entry := range []struct {
		name   string
		target **time.Time
		end    bool
	}{{"created_from", &q.CreatedFrom, false}, {"created_to", &q.CreatedTo, true}, {"transaction_from", &q.TransactionFrom, false}, {"transaction_to", &q.TransactionTo, true}} {
		value := ctx.Query(entry.name)
		if value == "" {
			continue
		}
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if len(value) == 10 {
			parsed, parseErr = time.ParseInLocation("2006-01-02", value, location)
			if parseErr == nil && entry.end {
				parsed = parsed.AddDate(0, 0, 1)
			}
		}
		if parseErr != nil {
			return q, invalid(entry.name, "must be a date or RFC3339 timestamp with timezone")
		}
		*entry.target = &parsed
	}
	if q.CreatedFrom != nil && q.CreatedTo != nil && !q.CreatedFrom.Before(*q.CreatedTo) {
		return q, invalid("created_to", "must be after created_from")
	}
	if q.TransactionFrom != nil && q.TransactionTo != nil && !q.TransactionFrom.Before(*q.TransactionTo) {
		return q, invalid("transaction_to", "must be after transaction_from")
	}
	return q, nil
}

func (h *PetroRunHandler) ListAdminOrders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "private, no-store")
	q, err := parsePetroRunAdminQuery(ctx)
	if err != nil {
		sendPetroRunError(ctx, "failed to list orders", err)
		return
	}
	items, total, err := h.service.ListAdminOrders(ctx.Request.Context(), q)
	if err != nil {
		sendPetroRunError(ctx, "failed to list orders", err)
		return
	}
	response.BuildResponseSuccess("orders retrieved", items, pagination.Meta{Page: q.Page, Take: q.PageSize, TotalData: int(total), TotalPage: (int(total) + q.PageSize - 1) / q.PageSize}).Send(ctx)
}

func (h *PetroRunHandler) SummarizeAdminOrders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "private, no-store")
	q, err := parsePetroRunAdminQuery(ctx)
	if err != nil {
		sendPetroRunError(ctx, "failed to summarize orders", err)
		return
	}
	result, err := h.service.SummarizeAdminOrders(ctx.Request.Context(), q)
	if err != nil {
		sendPetroRunError(ctx, "failed to summarize orders", err)
		return
	}
	response.BuildResponseSuccess("order summary retrieved", result).Send(ctx)
}

func (h *PetroRunHandler) GetAdminOrder(ctx *gin.Context) {
	ctx.Header("Cache-Control", "private, no-store")
	result, err := h.service.GetAdminOrder(ctx.Request.Context(), ctx.Param("order_id"))
	if err != nil {
		sendPetroRunError(ctx, "failed to get order", err)
		return
	}
	response.BuildResponseSuccess("order retrieved", result).Send(ctx)
}
