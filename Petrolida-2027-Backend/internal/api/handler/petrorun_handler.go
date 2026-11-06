package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	mylog "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
	pagination "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/meta"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PetroRunHandler struct {
	cfg     config.PetroRunConfig
	service service.PetroRunService
}

func NewPetroRun(c config.PetroRunConfig, service service.PetroRunService) *PetroRunHandler {
	return &PetroRunHandler{cfg: c, service: service}
}

func (h *PetroRunHandler) Config(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	if !h.cfg.Available() {
		response.BuildResponseFailed("PetroRun configuration unavailable", nil).SetStatus(http.StatusServiceUnavailable).Send(ctx)
		return
	}
	data := gin.H{"base_price_idr": h.cfg.BasePriceIDR, "admin_fee_idr": h.cfg.AdminFeeIDR, "bank": gin.H{"name": h.cfg.BankName, "account_number": h.cfg.BankAccountNumber, "account_holder": h.cfg.BankAccountHolder}, "available_sizes": h.cfg.AvailableSizes(), "max_participants": 4, "payment_window_seconds": 86400, "upload_policy": gin.H{"max_bytes": 5242880, "mime_types": []string{"image/jpeg", "image/png", "application/pdf"}}, "server_time": time.Now().UTC()}
	data["test_mode"] = h.cfg.TestMode
	if h.cfg.Sales != nil {
		availability, err := h.service.SaleAvailability(ctx.Request.Context())
		if err != nil {
			sendPetroRunError(ctx, "failed to get sale availability", err)
			return
		}
		data["sales"], data["availability"] = h.cfg.Sales, availability
	}
	response.BuildResponseSuccess("Success", data, pagination.Meta{}).Send(ctx)
}

func (h *PetroRunHandler) CreateOrder(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")

	key, err := uuid.Parse(ctx.GetHeader("Idempotency-Key"))
	if err != nil {
		sendPetroRunError(ctx, "failed to create order", myerror.NewFieldValidation("idempotency_key", "Idempotency-Key", "header must be a valid UUID"))
		return
	}
	var input domain.CreatePetroRunOrderInput
	if err := ctx.ShouldBindBodyWithJSON(&input); err != nil {
		sendPetroRunError(ctx, "failed to create order", myerror.NewFieldValidation("body", "Order", "must be valid JSON"))
		return
	}

	result, err := h.service.CreateOrder(ctx.Request.Context(), key, input)
	if err != nil {
		sendPetroRunError(ctx, "failed to create order", err)
		return
	}
	response.BuildResponseSuccess("order created", result).SetStatus(http.StatusCreated).Send(ctx)
}

// ListTickets untuk link di halaman akhir. Access token dikirim lewat header
// Authorization, bukan query string, supaya tidak tercatat di access log.
func (h *PetroRunHandler) ListTickets(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")

	token, ok := strings.CutPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		sendPetroRunError(ctx, "failed to get tickets", domain.ErrPetroRunInvalidAccess)
		return
	}

	result, err := h.service.ListTickets(ctx.Request.Context(), ctx.Param("order_id"), strings.TrimSpace(token))
	if err != nil {
		sendPetroRunError(ctx, "failed to get tickets", err)
		return
	}
	response.BuildResponseSuccess("tickets retrieved", result).Send(ctx)
}

// LookupTicket untuk fitur cek tiket di halaman awal.
func (h *PetroRunHandler) LookupTicket(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")

	result, err := h.service.LookupTicket(ctx.Request.Context(), ctx.Param("ticket_number"))
	if err != nil {
		sendPetroRunError(ctx, "failed to get ticket", err)
		return
	}
	response.BuildResponseSuccess("ticket retrieved", result).Send(ctx)
}

// VerifyTicket menerima isi QR lewat body (POST), supaya token QR tidak
// tercatat di access log seperti kalau ditaruh di path.
func (h *PetroRunHandler) VerifyTicket(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")

	var input domain.VerifyPetroRunTicketInput
	if err := ctx.ShouldBindBodyWithJSON(&input); err != nil {
		sendPetroRunError(ctx, "failed to verify ticket", myerror.NewFieldValidation("body", "QR payload", "must be valid JSON"))
		return
	}

	result, err := h.service.VerifyTicket(ctx.Request.Context(), input.QRPayload)
	if err != nil {
		sendPetroRunError(ctx, "failed to verify ticket", err)
		return
	}
	response.BuildResponseSuccess("ticket verified", result).Send(ctx)
}

func (h *PetroRunHandler) SubmitPaymentProof(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, (5<<20)+(128<<10))
	if err := ctx.Request.ParseMultipartForm(128 << 10); err != nil {
		handleMultipartError(ctx, err)
		return
	}
	if ctx.Request.MultipartForm != nil {
		defer ctx.Request.MultipartForm.RemoveAll()
		if len(ctx.Request.MultipartForm.File) != 1 ||
			len(ctx.Request.MultipartForm.File["proof"]) != 1 ||
			len(ctx.Request.MultipartForm.Value) != 1 ||
			len(ctx.Request.MultipartForm.Value["ocr"]) != 1 {
			sendPetroRunError(ctx, "failed to upload payment proof", myerror.NewFieldValidation("multipart", "Multipart form", "must contain exactly one proof file and one OCR field"))
			return
		}
	}
	file, header, err := ctx.Request.FormFile("proof")
	if err != nil {
		sendPetroRunError(ctx, "failed to upload payment proof", myerror.NewFieldValidation("proof", "Payment proof", "multipart field is required"))
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil {
		handleMultipartError(ctx, err)
		return
	}
	if len(content) > 5<<20 {
		handleMultipartError(ctx, &http.MaxBytesError{Limit: 5 << 20})
		return
	}
	var input domain.PetroRunOCRInput
	ocr := []byte(ctx.Request.FormValue("ocr"))
	if len(ocr) == 0 || len(ocr) > 64<<10 {
		sendPetroRunError(ctx, "failed to upload payment proof", myerror.NewFieldValidation("ocr", "OCR metadata", "JSON field is required and must not exceed 64 KB"))
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(ocr))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		sendPetroRunError(ctx, "failed to upload payment proof", myerror.NewFieldValidation("ocr", "OCR metadata", "must be valid JSON"))
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		sendPetroRunError(ctx, "failed to upload payment proof", myerror.NewFieldValidation("ocr", "OCR metadata", "must contain exactly one JSON object"))
		return
	}

	token, ok := strings.CutPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		sendPetroRunError(ctx, "failed to upload payment proof", domain.ErrPetroRunInvalidAccess)
		return
	}
	result, err := h.service.SubmitPaymentProof(ctx.Request.Context(), ctx.Param("order_id"), strings.TrimSpace(token), input, header.Filename, content)
	if err != nil {
		sendPetroRunError(ctx, "failed to upload payment proof", err)
		return
	}
	response.BuildResponseSuccess("payment proof submitted", result).SetStatus(http.StatusCreated).Send(ctx)
}

func (h *PetroRunHandler) ListPaymentProofs(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	page, pageSize, err := parsePagination(ctx, 20, "page_size", "take")
	if err != nil {
		sendPetroRunError(ctx, "failed to list payment proofs", err)
		return
	}
	status := strings.TrimSpace(ctx.Query("status"))
	if status == "" {
		status = "pending_review"
	}
	switch status {
	case "pending_review", "approved", "rejected", "rejected_non_receipt":
	default:
		sendPetroRunError(ctx, "failed to list payment proofs", myerror.NewFieldValidation("status", "Status", "is not a valid payment proof status"))
		return
	}
	items, total, err := h.service.ListPaymentProofs(ctx.Request.Context(), status, page, pageSize)
	if err != nil {
		sendPetroRunError(ctx, "failed to list payment proofs", err)
		return
	}
	meta := pagination.Meta{Page: page, Take: pageSize, TotalData: int(total), TotalPage: (int(total) + pageSize - 1) / pageSize}
	response.BuildResponseSuccess("payment proofs retrieved", items, meta).Send(ctx)
}

func (h *PetroRunHandler) GetPaymentProofFile(ctx *gin.Context) {
	ctx.Header("Cache-Control", "private, no-store")
	ctx.Header("X-Content-Type-Options", "nosniff")
	proofID, ok := parseUUIDParam(ctx, "proof_id", "Proof ID", "failed to get payment proof")
	if !ok {
		return
	}
	file, mimeType, err := h.service.GetPaymentProofFile(ctx.Request.Context(), proofID)
	if err != nil {
		sendPetroRunError(ctx, "failed to get payment proof", err)
		return
	}
	defer file.Close()
	ctx.Header("Content-Type", mimeType)
	disposition, extension := "attachment", ".pdf"
	switch mimeType {
	case "image/jpeg":
		disposition, extension = "inline", ".jpg"
	case "image/png":
		disposition, extension = "inline", ".png"
	}
	ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"payment-proof-%s%s\"", disposition, proofID, extension))
	ctx.Status(http.StatusOK)
	if _, err := io.Copy(ctx.Writer, file); err != nil {
		mylog.Errorf("petrorun failed to stream payment proof %s: %v", proofID, err)
	}
}

func (h *PetroRunHandler) ReviewPaymentProof(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	proofID, ok := parseUUIDParam(ctx, "proof_id", "Proof ID", "failed to review payment proof")
	if !ok {
		return
	}
	profile, ok := adminProfileFromContext(ctx)
	if !ok {
		sendPetroRunError(ctx, "failed to review payment proof", domain.ErrUnauthenticated)
		return
	}
	var input domain.ReviewPetroRunPaymentProofInput
	if err := ctx.ShouldBindBodyWithJSON(&input); err != nil {
		sendPetroRunError(ctx, "failed to review payment proof", myerror.NewFieldValidation("body", "Review", "must be valid JSON"))
		return
	}
	if input.Decision != "approve" && input.Decision != "reject" {
		sendPetroRunError(ctx, "failed to review payment proof", myerror.NewFieldValidation("decision", "Decision", "must be approve or reject"))
		return
	}
	result, err := h.service.ReviewPaymentProof(ctx.Request.Context(), proofID, input.Decision == "approve", profile.ID, input.Note)
	if err != nil {
		sendPetroRunError(ctx, "failed to review payment proof", err)
		return
	}
	response.BuildResponseSuccess("payment proof reviewed", result).Send(ctx)
}

func sendPetroRunError(ctx *gin.Context, message string, err error) {
	var typed myerror.Error
	if !errors.As(err, &typed) {
		// Error mentah (misalnya dari Postgres) dicatat di log, bukan dikirim ke client.
		mylog.Errorf("petrorun %s: %v", message, err)
		err = domain.ErrGeneral
	}

	result := response.BuildResponseFailed(message, err)
	switch {
	case typed.Type == myerror.TypeValidation:
		result = result.SetStatus(http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrPetroRunUnavailable):
		result = result.SetStatus(http.StatusServiceUnavailable)
	}
	result.Send(ctx)
}
