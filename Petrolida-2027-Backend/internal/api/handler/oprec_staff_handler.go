package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/export"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OprecStaffHandler struct {
	service service.OprecStaffService
}

func NewOprecStaff(oprecService service.OprecStaffService) *OprecStaffHandler {
	return &OprecStaffHandler{service: oprecService}
}

func (h *OprecStaffHandler) Config(ctx *gin.Context) {
	response.BuildResponseSuccess("oprec staff configuration", h.service.Config(time.Now())).Send(ctx)
}

func (h *OprecStaffHandler) ConfirmRegistration(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	var input struct {
		SubmissionKey string `json:"submission_key"`
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1024)
	if err := ctx.ShouldBindJSON(&input); err != nil {
		sendOprecError(ctx, "invalid submission confirmation", myerror.NewFieldValidation("submission_key", "Submission", "must be valid JSON"))
		return
	}
	result, err := h.service.ConfirmRegistration(ctx.Request.Context(), input.SubmissionKey)
	if err != nil {
		sendOprecError(ctx, "submission confirmation unavailable", err)
		return
	}
	response.BuildResponseSuccess("submission confirmation", result).Send(ctx)
}

func (h *OprecStaffHandler) Register(ctx *gin.Context) {
	data := ctx.PostForm("data")
	if data == "" {
		if err := ctx.Request.ParseMultipartForm(12 << 20); err != nil {
			handleMultipartError(ctx, err)
			return
		}
		data = ctx.Request.FormValue("data")
	}
	var input domain.RegistrationInput
	if err := json.Unmarshal([]byte(data), &input); err != nil {
		sendOprecError(ctx, "invalid registration data", myerror.NewFieldValidation("data", "Registration data", "must be valid JSON"))
		return
	}
	portfolio, err := ctx.FormFile("portfolio")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		handleMultipartError(ctx, err)
		return
	}
	result, err := h.service.Register(input, portfolio)
	if err != nil {
		sendOprecError(ctx, "registration failed", err)
		return
	}
	response.BuildResponseSuccess("registration submitted", result).SetStatus(http.StatusCreated).Send(ctx)
}

func (h *OprecStaffHandler) List(ctx *gin.Context) {
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	params, err := parseListParams(ctx)
	if err != nil {
		sendOprecError(ctx, "invalid list parameters", err)
		return
	}
	items, meta, err := h.service.List(admin, params)
	if err != nil {
		sendOprecError(ctx, "failed to get registrants", err)
		return
	}
	response.BuildResponseSuccess("registrants retrieved", items).WithMeta(meta).Send(ctx)
}

// ExportCSV dan ExportExcel mengunduh seluruh hasil filter daftar pendaftar.
// Parameter page dan page_size tetap divalidasi oleh parseListParams tetapi tidak dipakai.
func (h *OprecStaffHandler) ExportCSV(ctx *gin.Context) {
	h.exportRegistrants(ctx, "csv", "text/csv; charset=utf-8", export.WriteCSV)
}

func (h *OprecStaffHandler) ExportExcel(ctx *gin.Context) {
	h.exportRegistrants(
		ctx, "xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", export.WriteXLSX,
	)
}

func (h *OprecStaffHandler) exportRegistrants(
	ctx *gin.Context,
	extension, contentType string,
	write func(io.Writer, export.Table) error,
) {
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	params, err := parseListParams(ctx)
	if err != nil {
		sendOprecError(ctx, "invalid export parameters", err)
		return
	}
	table, err := h.service.Export(admin, params)
	if err != nil {
		sendOprecError(ctx, "failed to export registrants", err)
		return
	}
	// Ditulis ke buffer dulu supaya kegagalan masih bisa dikirim sebagai error
	// JSON, bukan file yang terpotong dengan status 200.
	var buffer bytes.Buffer
	if err := write(&buffer, table); err != nil {
		sendOprecError(ctx, "failed to export registrants", domain.ErrGeneral)
		return
	}
	filename := fmt.Sprintf("oprec-staff-%s.%s", time.Now().In(export.Jakarta).Format("20060102-1504"), extension)
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Data(http.StatusOK, contentType, buffer.Bytes())
}

func (h *OprecStaffHandler) Detail(ctx *gin.Context) {
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	id, ok := parseUUIDParam(ctx, "id", "Registrant", "invalid registrant id")
	if !ok {
		return
	}
	registrant, err := h.service.GetByID(admin, id)
	if err != nil {
		sendOprecError(ctx, "failed to get registrant", err)
		return
	}
	response.BuildResponseSuccess("registrant retrieved", registrant).Send(ctx)
}

func (h *OprecStaffHandler) UpdateStatus(ctx *gin.Context) {
	registrantID, ok := parseUUIDParam(ctx, "id", "Registrant", "invalid registrant id")
	if !ok {
		return
	}
	var input domain.UpdateRegistrantStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		sendOprecError(ctx, "invalid status update", myerror.NewFieldValidation("body", "Status update", "must be valid JSON"))
		return
	}
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	result, err := h.service.UpdateStatus(admin, registrantID, input)
	if err != nil {
		sendOprecError(ctx, "failed to update registrant status", err)
		return
	}
	response.BuildResponseSuccess("registrant status updated", result).Send(ctx)
}

func (h *OprecStaffHandler) Document(ctx *gin.Context) {
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	registrantID, ok := parseUUIDParam(ctx, "id", "Registrant", "invalid registrant id")
	if !ok {
		return
	}
	documentID, ok := parseUUIDParam(ctx, "document_id", "Document", "invalid document id")
	if !ok {
		return
	}
	document, file, err := h.service.OpenPortfolio(admin, registrantID, documentID)
	if err != nil {
		sendOprecError(ctx, "failed to get document", err)
		return
	}
	defer file.Close()
	filename := "portfolio.pdf"
	if document.OriginalName != nil && *document.OriginalName != "" {
		filename = strings.ReplaceAll(*document.OriginalName, `"`, "")
	}
	size := int64(-1)
	if document.SizeBytes != nil {
		size = *document.SizeBytes
	}
	ctx.DataFromReader(http.StatusOK, size, "application/pdf", file, map[string]string{
		"Content-Disposition": fmt.Sprintf(`inline; filename="%s"`, filename),
		"Cache-Control":       "private, no-store",
	})
}

func (h *OprecStaffHandler) Counts(ctx *gin.Context) {
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	counts, err := h.service.Counts(admin)
	if err != nil {
		sendOprecError(ctx, "failed to count registrants", err)
		return
	}
	response.BuildResponseSuccess("registrant counts retrieved", counts).Send(ctx)
}

func (h *OprecStaffHandler) PrimarySubdivisionCounts(ctx *gin.Context) {
	admin, ok := requireAdmin(ctx)
	if !ok {
		return
	}
	counts, err := h.service.PrimarySubdivisionCounts(admin)
	if err != nil {
		sendOprecError(ctx, "failed to count subdivisions", err)
		return
	}
	response.BuildResponseSuccess("subdivision counts retrieved", counts).Send(ctx)
}

func (h *OprecStaffHandler) ActivityLogs(ctx *gin.Context) {
	params, err := parseActivityLogParams(ctx)
	if err != nil {
		sendOprecError(ctx, "invalid activity log parameters", err)
		return
	}
	logs, meta, err := h.service.ActivityLogs(params)
	if err != nil {
		sendOprecError(ctx, "failed to get activity logs", err)
		return
	}
	response.BuildResponseSuccess("activity logs retrieved", logs).WithMeta(meta).Send(ctx)
}

func parseActivityLogParams(ctx *gin.Context) (domain.ActivityLogListParams, error) {
	var params domain.ActivityLogListParams
	var err error
	params.Page, params.PageSize, err = parsePagination(ctx, 20, "page_size")
	if err != nil {
		return params, err
	}
	err = parseUUIDQueries(ctx, map[string]**uuid.UUID{
		"actor_id": &params.ActorID, "division_id": &params.DivisionID,
	})
	return params, err
}

func parseListParams(ctx *gin.Context) (domain.RegistrantListParams, error) {
	params := domain.RegistrantListParams{
		Search: strings.TrimSpace(ctx.Query("search")),
		Status: strings.TrimSpace(ctx.Query("status")), SortBy: ctx.DefaultQuery("sort_by", "created_at"),
		SortDirection: ctx.DefaultQuery("sort", "desc"),
	}
	var err error
	// "take" adalah nama lama page_size yang masih didukung.
	params.Page, params.PageSize, err = parsePagination(ctx, 10, "page_size", "take")
	if err != nil {
		return params, err
	}
	allowedSort := map[string]bool{"created_at": true, "name": true, "nrp": true, "status": true}
	if !allowedSort[params.SortBy] {
		return params, myerror.NewFieldValidation("sort_by", "Sort field", "is not supported")
	}
	if params.SortDirection != "asc" && params.SortDirection != "desc" {
		return params, myerror.NewFieldValidation("sort", "Sort direction", "must be asc or desc")
	}
	if params.Status != "" && !entity.OprecStaffStatus(params.Status).IsValid() {
		return params, myerror.NewFieldValidation("status", "Status", "is not supported")
	}
	if err := parseUUIDQueries(ctx, map[string]**uuid.UUID{
		"faculty_id": &params.FacultyID, "department_id": &params.DepartmentID,
		"subdivision_id": &params.SubdivisionID, "division_id": &params.DivisionID,
	}); err != nil {
		return params, err
	}
	if value := strings.TrimSpace(ctx.Query("choice_priority")); value != "" && value != "all" {
		if value != "1" && value != "2" {
			return params, myerror.NewFieldValidation("choice_priority", "Choice priority", "must be 1, 2, or all")
		}
		parsed, _ := strconv.ParseInt(value, 10, 16)
		params.ChoicePriority = int16(parsed)
	}
	if params.ChoicePriority != 0 && params.SubdivisionID == nil && params.DivisionID == nil {
		return params, myerror.NewFieldValidation("choice_priority", "Choice priority", "requires division_id or subdivision_id")
	}
	if value := strings.TrimSpace(ctx.Query("batch")); value != "" {
		parsed, parseErr := strconv.ParseInt(value, 10, 16)
		if parseErr != nil {
			return params, myerror.NewFieldValidation("batch", "Batch", "must be a valid year")
		}
		batch := int16(parsed)
		params.Batch = &batch
	}
	return params, nil
}
