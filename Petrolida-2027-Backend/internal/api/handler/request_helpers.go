package handler

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/middleware"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requireAdmin mengambil profil admin dari context. Kalau tidak ada, response
// 401 langsung dikirim dan handler cukup return.
func requireAdmin(ctx *gin.Context) (domain.AdminProfile, bool) {
	admin, ok := adminProfileFromContext(ctx)
	if !ok {
		sendOprecError(ctx, "authentication required", domain.ErrUnauthenticated)
	}
	return admin, ok
}

func adminProfileFromContext(ctx *gin.Context) (domain.AdminProfile, bool) {
	value, ok := ctx.Get(middleware.AdminProfileContextKey)
	if !ok {
		return domain.AdminProfile{}, false
	}
	profile, ok := value.(domain.AdminProfile)
	return profile, ok
}

// parseUUIDParam mem-parse path parameter UUID. Kalau tidak valid, response
// error langsung dikirim dan handler cukup return.
func parseUUIDParam(ctx *gin.Context, param, label, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(ctx.Param(param))
	if err != nil {
		sendOprecError(ctx, message, myerror.NewFieldValidation(param, label, "must be a valid UUID"))
		return uuid.Nil, false
	}
	return id, true
}

// parsePagination membaca query page dan page size. pageSizeKeys dicek berurutan,
// yang pertama terisi yang dipakai (misalnya "page_size" lalu alias lama "take").
func parsePagination(ctx *gin.Context, defaultPageSize int, pageSizeKeys ...string) (int, int, error) {
	page, pageSize := 1, defaultPageSize
	var err error
	if value := ctx.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return page, pageSize, myerror.NewFieldValidation("page", "Page", "must be a positive integer")
		}
	}
	pageSizeValue := ""
	for _, key := range pageSizeKeys {
		if pageSizeValue = ctx.Query(key); pageSizeValue != "" {
			break
		}
	}
	if pageSizeValue != "" {
		pageSize, err = strconv.Atoi(pageSizeValue)
		if err != nil || pageSize < 1 || pageSize > 100 {
			return page, pageSize, myerror.NewFieldValidation("page_size", "Page size", "must be between 1 and 100")
		}
	}
	return page, pageSize, nil
}

// parseUUIDQueries mengisi setiap target dengan UUID dari query parameter yang
// namanya sama dengan key map. Query yang kosong dilewati.
func parseUUIDQueries(ctx *gin.Context, targets map[string]**uuid.UUID) error {
	for key, target := range targets {
		if value := strings.TrimSpace(ctx.Query(key)); value != "" {
			id, err := uuid.Parse(value)
			if err != nil {
				return myerror.NewFieldValidation(key, key, "must be a valid UUID")
			}
			*target = &id
		}
	}
	return nil
}

func sendOprecError(ctx *gin.Context, message string, err error) {
	result := response.BuildResponseFailed(message, err)
	var typed myerror.Error
	if errors.As(err, &typed) && typed.Type == myerror.TypeValidation {
		result = result.SetStatus(http.StatusUnprocessableEntity)
	}
	result.Send(ctx)
}

func handleMultipartError(ctx *gin.Context, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) || errors.Is(err, io.EOF) || errors.Is(err, multipart.ErrMessageTooLarge) {
		response.BuildResponseFailed("request is too large", myerror.New(myerror.TypeValidation, "PAYLOAD_TOO_LARGE", "request is too large")).
			SetStatus(http.StatusRequestEntityTooLarge).Send(ctx)
		return
	}
	sendOprecError(ctx, "invalid multipart request", err)
}
