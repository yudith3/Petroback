package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/storage"
	"github.com/google/uuid"
)

const (
	maxParticipants          = 4
	maxPaymentTime           = 24 * time.Hour
	maxPaymentProofBytes     = 5 << 20
	autoApproveOCRConfidence = 0.90
	// Bentrok ticket number acak hampir mustahil, tiga kali percobaan sudah cukup.
	maxApproveAttempts = 3
	// Prefix isi QR supaya scanner bisa membedakan QR tiket PetroRun dari QR lain.
	petroRunQRPrefix = "PR27T:"
)

type PetroRunService interface {
	SaleAvailability(ctx context.Context) (domain.PetroRunSaleAvailability, error)
	GetOrder(ctx context.Context, orderNumber, token string) (domain.PetroRunOrderResponse, error)
	ListAdminOrders(ctx context.Context, query domain.PetroRunAdminQuery) ([]domain.PetroRunAdminOrder, int64, error)
	GetAdminOrder(ctx context.Context, orderNumber string) (domain.PetroRunAdminOrder, error)
	SummarizeAdminOrders(ctx context.Context, query domain.PetroRunAdminQuery) (domain.PetroRunSummary, error)
	CreateOrder(ctx context.Context, key uuid.UUID, input domain.CreatePetroRunOrderInput) (domain.PetroRunOrderResponse, error)
	ApproveOrder(ctx context.Context, orderNumber string, actorID *uuid.UUID) (domain.PetroRunOrderTicketsResponse, error)
	ListTickets(ctx context.Context, orderNumber, token string) (domain.PetroRunOrderTicketsResponse, error)
	LookupTicket(ctx context.Context, ticketNumber string) (domain.PetroRunTicketResponse, error)
	VerifyTicket(ctx context.Context, qrPayload string) (domain.PetroRunTicketResponse, error)
	SubmitPaymentProof(ctx context.Context, orderNumber, accessToken string, input domain.PetroRunOCRInput, filename string, content []byte) (domain.PetroRunPaymentProofResponse, error)
	ListPaymentProofs(ctx context.Context, status string, page, pageSize int) ([]domain.PetroRunPaymentProofReviewItem, int64, error)
	GetPaymentProofFile(ctx context.Context, proofID uuid.UUID) (io.ReadCloser, string, error)
	ReviewPaymentProof(ctx context.Context, proofID uuid.UUID, approve bool, reviewerID uuid.UUID, note string) (domain.PetroRunPaymentProofResponse, error)
}

type petroRunService struct {
	repository repository.PetroRunRepository
	cfg        config.PetroRunConfig
	proofStore storage.Store
	now        func() time.Time
}

func NewPetroRunService(repository repository.PetroRunRepository, cfg config.PetroRunConfig, proofStores ...storage.Store) PetroRunService {
	var proofStore storage.Store
	if len(proofStores) > 0 {
		proofStore = proofStores[0]
	}
	return &petroRunService{repository: repository, cfg: cfg, proofStore: proofStore, now: time.Now}
}

func (s *petroRunService) CreateOrder(ctx context.Context, key uuid.UUID, input domain.CreatePetroRunOrderInput) (domain.PetroRunOrderResponse, error) {
	if !s.cfg.Available() {
		return domain.PetroRunOrderResponse{}, domain.ErrPetroRunUnavailable
	}
	if key == uuid.Nil {
		return domain.PetroRunOrderResponse{}, myerror.NewFieldValidation("idempotency_key", "Idempotency key", "must be a valid UUID")
	}

	// Retry dengan key yang sama mengembalikan order yang sudah ada,
	// jadi FE aman mengirim ulang saat koneksi putus.
	existing, err := s.repository.GetByIdempotencyIdKey(ctx, key)
	if err == nil {
		return s.toResponse(existing), nil
	}
	if !errors.Is(err, domain.ErrPetroRunOrderNotFound) {
		return domain.PetroRunOrderResponse{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)

	participants, surcharge, err := validatePetroRunInput(&input, s.cfg)
	if err != nil {
		return domain.PetroRunOrderResponse{}, err
	}
	if s.cfg.Sales != nil && s.cfg.Sales.RegistrationStatus(now) != "open" {
		return domain.PetroRunOrderResponse{}, domain.ErrPetroRunRegistrationClosed
	}

	// Harga dihitung ulang dari config, nominal dari client tidak dipakai.
	participantSubtotal := s.cfg.BasePriceIDR*int64(len(participants)) + surcharge
	baseTotal := participantSubtotal + s.cfg.AdminFeeIDR

	// CreatedAt diisi sendiri supaya expires_at tepat created_at + 24 jam.
	// Dipotong ke mikrodetik (presisi Postgres) supaya response pertama dan
	// response retry menampilkan waktu yang sama.
	order := entity.PetroRunOrder{
		RegistrantName:         input.FullName,
		RegistrantEmail:        input.Email,
		RegistrantPhone:        input.WhatsappNumber,
		Status:                 entity.PetroRunWaitingPayment,
		BasePriceIDR:           s.cfg.BasePriceIDR,
		AdminFeeIDR:            s.cfg.AdminFeeIDR,
		ParticipantSubtotalIDR: participantSubtotal,
		BankName:               s.cfg.BankName,
		BankAccountNumber:      s.cfg.BankAccountNumber,
		BankAccountHolder:      s.cfg.BankAccountHolder,
		ExpiresAt:              now.Add(maxPaymentTime),
		Participants:           participants,
		IdempotencyKey:         &key,
	}
	order.CreatedAt = now

	if s.cfg.Sales != nil {
		rules := *s.cfg.Sales
		rules.NormalPriceIDR = s.cfg.BasePriceIDR
		err = s.repository.CreatePricedOrder(ctx, now, rules, surcharge, &order)
	} else {
		err = s.repository.CreateOrder(ctx, now, baseTotal, &order)
	}
	if errors.Is(err, domain.ErrPetroRunConflict) {
		// Dua request dengan key yang sama datang bersamaan: yang kalah
		// mengembalikan order milik yang menang.
		if existing, getErr := s.repository.GetByIdempotencyIdKey(ctx, key); getErr == nil {
			return s.toResponse(existing), nil
		}
	}
	if err != nil {
		return domain.PetroRunOrderResponse{}, err
	}
	return s.toResponse(order), nil
}

func validatePetroRunInput(input *domain.CreatePetroRunOrderInput, cfg config.PetroRunConfig) ([]entity.PetroRunParticipant, int64, error) {
	input.FullName = strings.TrimSpace(input.FullName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	fields := make([]myerror.FieldError, 0)
	add := func(field, label, message string) {
		fields = append(fields, myerror.FieldError{Field: field, Label: label, Message: message})
	}

	if len(input.FullName) < 3 || len(input.FullName) > 150 {
		add("full_name", "Full name", "must contain 3 to 150 characters")
	}
	if address, err := mail.ParseAddress(input.Email); err != nil || !strings.EqualFold(address.Address, input.Email) {
		add("email", "Email", "must be a valid email address")
	}
	whatsapp, ok := nomarlizeWhatsapp(input.WhatsappNumber)
	if !ok {
		add("whatsapp_number", "WhatsApp number", "must be a valid Indonesian mobile number")
	}
	input.WhatsappNumber = whatsapp
	if !input.AcceptTerms {
		add("accept_terms", "Terms", "must be accepted")
	}
	if !input.AcceptPrivacy {
		add("accept_privacy", "Privacy policy", "must be accepted")
	}
	if len(input.Participants) < 1 || len(input.Participants) > maxParticipants {
		add("participants", "Participants", fmt.Sprintf("must contain 1 to %d participants", maxParticipants))
	}

	participants := make([]entity.PetroRunParticipant, 0, len(input.Participants))
	var surcharge int64
	for index := range input.Participants {
		participant := &input.Participants[index]
		participant.FullName = strings.TrimSpace(participant.FullName)
		participant.PaceTarget = strings.TrimSpace(participant.PaceTarget)
		participant.MedicalHistory = strings.TrimSpace(participant.MedicalHistory)
		prefix := fmt.Sprintf("participants.%d", index)

		if len(participant.FullName) < 3 || len(participant.FullName) > 150 {
			add(prefix+".full_name", "Participant name", "must contain 3 to 150 characters")
		}
		size, ok := cfg.JerseySize(participant.JerseySize)
		if !ok {
			add(prefix+".jersey_size", "Jersey size", "must be one of S, M, L, XL, XXL/2XL, 3XL, or 4XL")
		}
		if participant.PaceTarget == "" || len(participant.PaceTarget) > 50 {
			add(prefix+".pace_target", "Pace target", "is required and must not exceed 50 characters")
		}
		// "tidak ada" tetap valid, yang ditolak hanya isian kosong.
		if participant.MedicalHistory == "" || len(participant.MedicalHistory) > 2000 {
			add(prefix+".medical_history", "Medical history", "is required (fill \"tidak ada\" if none) and must not exceed 2000 characters")
		}

		surcharge += size.SurchargeIDR
		// Email dan Contact peserta belum dikirim FE (lihat tiket), sementara
		// diisi kontak pendaftar. BirthDate menunggu keputusan tim.
		participants = append(participants, entity.PetroRunParticipant{
			Name:           participant.FullName,
			Email:          input.Email,
			Contact:        whatsapp,
			JerseySize:     size.Code,
			MedicalHistory: participant.MedicalHistory,
			RunningPace:    participant.PaceTarget,
		})
	}

	if len(fields) > 0 {
		return nil, 0, myerror.NewValidation(fields)
	}
	return participants, surcharge, nil
}

func nomarlizeWhatsapp(input string) (string, bool) {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, input)

	switch {
	case strings.HasPrefix(digits, "08"):
		digits = "62" + digits[1:]
	case strings.HasPrefix(digits, "8"):
		digits = "62" + digits
	}
	if !strings.HasPrefix(digits, "628") || len(digits) < 10 || len(digits) > 15 {
		return "", false
	}
	return "+" + digits, true
}

func (s *petroRunService) toResponse(order entity.PetroRunOrder) domain.PetroRunOrderResponse {
	baseSubtotal := order.BasePriceIDR * int64(len(order.Participants))
	if order.BaseSubtotalIDR > 0 {
		baseSubtotal = order.BaseSubtotalIDR
	}
	return domain.PetroRunOrderResponse{
		OrderID: order.OrderNumber,
		Status:  string(order.Status),
		Pricing: domain.PetroRunPricing{
			Lines:            petroRunPriceLines(order),
			BaseSubtotal:     baseSubtotal,
			JerseySurcharge:  order.ParticipantSubtotalIDR - baseSubtotal,
			AdminFee:         order.AdminFeeIDR,
			UniqueCode:       order.UniqueCode,
			UniqueAdjustment: order.UniqueAdjustmentIDR,
		},
		PayableTotal: order.PayableTotalIDR,
		Bank: domain.PetroRunBank{
			Name:          order.BankName,
			AccountNumber: order.BankAccountNumber,
			AccountHolder: order.BankAccountHolder,
		},
		CreatedAt:   order.CreatedAt.UTC(),
		ExpiresAt:   order.ExpiresAt.UTC(),
		ServerTime:  s.now().UTC(),
		AccessToken: s.accessToken(order.ID),
	}
}

// Token = HMAC(secret, order ID). Tidak perlu disimpan di DB, dan selalu sama
// untuk order yang sama, jadi retry mengembalikan token yang sama.
func (s *petroRunService) accessToken(orderID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.AccessSecret))
	mac.Write(orderID[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ApproveOrder dipakai autoapprove (actorID nil) dan approve manual admin.
// Approve ulang atau bersamaan mengembalikan tiket yang sama.
func (s *petroRunService) ApproveOrder(ctx context.Context, orderNumber string, actorID *uuid.UUID) (domain.PetroRunOrderTicketsResponse, error) {
	order, err := s.repository.GetByOrderNumber(ctx, normalizePetroRunCode(orderNumber))
	if err != nil {
		return domain.PetroRunOrderTicketsResponse{}, err
	}

	// Conflict di sini berarti ticket number acak bentrok, transaksi sudah
	// di-rollback, jadi aman diulang dengan nomor baru.
	for attempt := 0; attempt < maxApproveAttempts; attempt++ {
		err = s.repository.ApproveOrder(ctx, order.ID, actorID, s.newTicket)
		if !errors.Is(err, domain.ErrPetroRunConflict) {
			break
		}
	}
	if err != nil {
		return domain.PetroRunOrderTicketsResponse{}, err
	}

	// Dibaca ulang supaya approve pertama dan approve ulang mengembalikan
	// data yang sama persis dari database.
	approved, err := s.repository.GetOrderWithTickets(ctx, order.OrderNumber)
	if err != nil {
		return domain.PetroRunOrderTicketsResponse{}, err
	}
	return s.toTicketsResponse(approved), nil
}

// ListTickets untuk link di halaman akhir. Token dicek di sini, bukan di DB,
// karena access token tidak disimpan (lihat accessToken).
func (s *petroRunService) ListTickets(ctx context.Context, orderNumber, token string) (domain.PetroRunOrderTicketsResponse, error) {
	if !s.cfg.Available() {
		return domain.PetroRunOrderTicketsResponse{}, domain.ErrPetroRunUnavailable
	}

	order, err := s.repository.GetOrderWithTickets(ctx, normalizePetroRunCode(orderNumber))
	if errors.Is(err, domain.ErrPetroRunOrderNotFound) {
		return domain.PetroRunOrderTicketsResponse{}, domain.ErrPetroRunInvalidAccess
	}
	if err != nil {
		return domain.PetroRunOrderTicketsResponse{}, err
	}
	if !hmac.Equal([]byte(token), []byte(s.accessToken(order.ID))) {
		return domain.PetroRunOrderTicketsResponse{}, domain.ErrPetroRunInvalidAccess
	}
	return s.toTicketsResponse(order), nil
}

// LookupTicket untuk cek tiket di halaman awal. Hasilnya hanya satu tiket,
// bukan data registrant atau peserta lain dalam order yang sama.
func (s *petroRunService) LookupTicket(ctx context.Context, ticketNumber string) (domain.PetroRunTicketResponse, error) {
	ticketNumber = normalizePetroRunCode(ticketNumber)
	// Format salah langsung ditolak tanpa query ke DB.
	if !isValidPetroRunTicketNumber(ticketNumber) {
		return domain.PetroRunTicketResponse{}, domain.ErrPetroRunTicketNotFound
	}

	participant, err := s.repository.GetParticipantByTicketNumber(ctx, ticketNumber)
	if err != nil {
		return domain.PetroRunTicketResponse{}, err
	}
	return s.toTicketResponse(participant), nil
}

// VerifyTicket untuk hasil scan QR. Yang dicari hash-nya, karena token
// mentah tidak disimpan di DB.
func (s *petroRunService) VerifyTicket(ctx context.Context, qrPayload string) (domain.PetroRunTicketResponse, error) {
	token := strings.TrimPrefix(strings.TrimSpace(qrPayload), petroRunQRPrefix)
	// Token = base64 tanpa padding dari HMAC-SHA256 (32 byte), panjangnya selalu 43.
	if len(token) != base64.RawURLEncoding.EncodedLen(sha256.Size) {
		return domain.PetroRunTicketResponse{}, domain.ErrPetroRunTicketNotFound
	}

	participant, err := s.repository.GetParticipantByTicketTokenHash(ctx, hashPetroRunToken(token))
	if err != nil {
		return domain.PetroRunTicketResponse{}, err
	}
	return s.toTicketResponse(participant), nil
}

func (s *petroRunService) SubmitPaymentProof(
	ctx context.Context,
	orderNumber string,
	accessToken string,
	input domain.PetroRunOCRInput,
	filename string,
	content []byte,
) (domain.PetroRunPaymentProofResponse, error) {
	if !s.cfg.Available() {
		return domain.PetroRunPaymentProofResponse{}, domain.ErrPetroRunUnavailable
	}
	if s.proofStore == nil {
		return domain.PetroRunPaymentProofResponse{}, domain.ErrPetroRunUnavailable
	}
	order, err := s.repository.GetByOrderNumber(ctx, normalizePetroRunCode(orderNumber))
	if errors.Is(err, domain.ErrPetroRunOrderNotFound) {
		return domain.PetroRunPaymentProofResponse{}, domain.ErrPetroRunInvalidAccess
	}
	if err != nil {
		return domain.PetroRunPaymentProofResponse{}, err
	}
	if !hmac.Equal([]byte(accessToken), []byte(s.accessToken(order.ID))) {
		return domain.PetroRunPaymentProofResponse{}, domain.ErrPetroRunInvalidAccess
	}

	mimeType, err := validatePaymentProofFile(filename, content)
	if err != nil {
		return domain.PetroRunPaymentProofResponse{}, err
	}
	input, transactionAt, reasons, err := evaluatePaymentProofOCR(input, order, s.now())
	if err != nil {
		return domain.PetroRunPaymentProofResponse{}, err
	}

	digest := sha256.Sum256(content)
	key, err := s.proofStore.Save("petrorun-proof-"+uuid.NewString()+filepath.Ext(filename), int64(len(content)), bytes.NewReader(content))
	if err != nil {
		return domain.PetroRunPaymentProofResponse{}, fmt.Errorf("save PetroRun payment proof: %w", err)
	}
	proof := entity.PetroRunPaymentProof{
		FileKey:               key,
		FileSHA256:            hex.EncodeToString(digest[:]),
		MIMEType:              mimeType,
		OCRSource:             "browser_client",
		OCRClassification:     input.Classification,
		OCRRawText:            input.RawText,
		OCRSenderName:         input.SenderName,
		OCRRecipientName:      input.RecipientName,
		OCRTransactionStatus:  input.TransactionStatus,
		OCRMismatchReasons:    strings.Join(reasons, "\n"),
		OCRExtractedAmount:    input.ExtractedAmount,
		OCRDestinationAccount: input.DestinationAccount,
		OCRDestinationBank:    input.DestinationBank,
		OCRTransactionAt:      transactionAt,
		OCRTimezone:           input.Timezone,
		OCRPrecision:          input.Precision,
		OCRConfidence:         input.Confidence,
		OCRReference:          input.Reference,
		OCROrderID:            input.OrderID,
	}

	autoApprove := len(reasons) == 0 && input.Classification == "receipt" &&
		input.Confidence != nil && *input.Confidence >= autoApproveOCRConfidence &&
		transactionAt != nil && order.Status == entity.PetroRunWaitingPayment
	var record repository.PetroRunProofRecord
	for attempt := 0; attempt < maxApproveAttempts; attempt++ {
		record, err = s.repository.SubmitPaymentProof(ctx, normalizePetroRunCode(orderNumber), &proof, s.now().UTC(), autoApprove, s.newTicket)
		if !errors.Is(err, domain.ErrPetroRunConflict) {
			break
		}
	}
	if err != nil {
		if cleanupErr := s.proofStore.Delete(key); cleanupErr != nil {
			return domain.PetroRunPaymentProofResponse{}, errors.Join(err, fmt.Errorf("delete unreferenced PetroRun payment proof: %w", cleanupErr))
		}
		return domain.PetroRunPaymentProofResponse{}, err
	}
	if record.Proof.ID != proof.ID {
		if cleanupErr := s.proofStore.Delete(key); cleanupErr != nil {
			return domain.PetroRunPaymentProofResponse{}, fmt.Errorf("delete duplicate PetroRun payment proof: %w", cleanupErr)
		}
	}
	result := toPaymentProofResponse(record.Proof)
	result.OrderID = record.Order.OrderNumber
	return result, nil
}

func validatePaymentProofFile(filename string, content []byte) (string, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	if len(content) == 0 {
		return "", myerror.NewFieldValidation("proof", "Payment proof", "file must not be empty")
	}
	if len(content) > maxPaymentProofBytes {
		return "", myerror.NewFieldValidation("proof", "Payment proof", "file must not exceed 5 MB")
	}

	var expectedMIME string
	switch extension {
	case ".jpg", ".jpeg":
		expectedMIME = "image/jpeg"
	case ".png":
		expectedMIME = "image/png"
	case ".pdf":
		expectedMIME = "application/pdf"
	default:
		return "", myerror.NewFieldValidation("proof", "Payment proof", "file must be JPEG, PNG, or PDF")
	}

	detectedMIME := http.DetectContentType(content)
	if detectedMIME != expectedMIME {
		return "", myerror.NewFieldValidation("proof", "Payment proof", "file extension does not match its content")
	}
	if extension == ".pdf" && !bytes.HasPrefix(content, []byte("%PDF-")) {
		return "", myerror.NewFieldValidation("proof", "Payment proof", "file is not a valid PDF")
	}
	return expectedMIME, nil
}

func evaluatePaymentProofOCR(input domain.PetroRunOCRInput, order entity.PetroRunOrder, now time.Time) (domain.PetroRunOCRInput, *time.Time, []string, error) {
	input.Classification = strings.ToLower(strings.TrimSpace(input.Classification))
	if input.Classification == "" {
		input.Classification = "unknown"
	}
	if input.Classification != "receipt" && input.Classification != "not_receipt" && input.Classification != "unknown" {
		return input, nil, nil, myerror.NewFieldValidation("ocr.classification", "OCR classification", "must be receipt, not_receipt, or unknown")
	}
	if input.Confidence != nil && (*input.Confidence < 0 || *input.Confidence > 1) {
		return input, nil, nil, myerror.NewFieldValidation("ocr.confidence", "OCR confidence", "must be between 0 and 1")
	}
	if len(input.RawText) > 10000 || len(input.SenderName) > 200 || len(input.RecipientName) > 200 ||
		len(input.Reference) > 200 || len(input.OrderID) > 100 ||
		len(input.DestinationAccount) > 100 || len(input.DestinationBank) > 150 {
		return input, nil, nil, myerror.NewFieldValidation("ocr", "OCR metadata", "one or more OCR fields exceed the allowed length")
	}
	input.RawText = strings.TrimSpace(input.RawText)
	input.SenderName = strings.TrimSpace(input.SenderName)
	input.RecipientName = strings.TrimSpace(input.RecipientName)
	input.TransactionStatus = strings.ToLower(strings.TrimSpace(input.TransactionStatus))
	if input.TransactionStatus == "" {
		input.TransactionStatus = "unknown"
	}
	if input.TransactionStatus != "success" && input.TransactionStatus != "failed" && input.TransactionStatus != "unknown" {
		return input, nil, nil, myerror.NewFieldValidation("ocr.transaction_status", "Transaction status", "must be success, failed, or unknown")
	}
	input.DestinationAccount = strings.TrimSpace(input.DestinationAccount)
	input.DestinationBank = strings.TrimSpace(input.DestinationBank)
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Precision = strings.ToLower(strings.TrimSpace(input.Precision))
	input.Reference = strings.TrimSpace(input.Reference)
	input.OrderID = strings.TrimSpace(input.OrderID)
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "raw_text", value: input.RawText},
		{name: "sender_name", value: input.SenderName},
		{name: "recipient_name", value: input.RecipientName},
		{name: "transaction_status", value: input.TransactionStatus},
		{name: "destination_account", value: input.DestinationAccount},
		{name: "destination_bank", value: input.DestinationBank},
		{name: "transaction_at", value: input.TransactionAt},
		{name: "timezone", value: input.Timezone},
		{name: "precision", value: input.Precision},
		{name: "reference", value: input.Reference},
		{name: "order_id", value: input.OrderID},
	} {
		if strings.ContainsRune(field.value, '\x00') {
			return input, nil, nil, myerror.NewFieldValidation("ocr."+field.name, "OCR metadata", "must not contain NUL characters")
		}
	}

	var reasons []string
	var transactionAt *time.Time
	if input.Classification == "unknown" {
		reasons = append(reasons, "classification_uncertain")
	}
	if input.RawText == "" {
		reasons = append(reasons, "ocr_raw_text_missing")
	}
	if input.SenderName == "" {
		reasons = append(reasons, "sender_name_missing")
	}
	if input.RecipientName == "" {
		reasons = append(reasons, "recipient_name_missing")
	} else if normalizeProofText(input.RecipientName) != normalizeProofText(order.BankAccountHolder) {
		reasons = append(reasons, "recipient_name_mismatch")
	}
	switch input.TransactionStatus {
	case "failed":
		reasons = append(reasons, "transaction_failed")
	case "unknown":
		reasons = append(reasons, "transaction_status_unknown")
	}
	if input.Confidence == nil || *input.Confidence < autoApproveOCRConfidence {
		reasons = append(reasons, "low_or_missing_ocr_confidence")
	}
	if input.ExtractedAmount == nil {
		reasons = append(reasons, "amount_missing")
	} else if *input.ExtractedAmount != order.PayableTotalIDR {
		reasons = append(reasons, "amount_mismatch")
	}
	if input.DestinationBank == "" || normalizeProofText(input.DestinationBank) != normalizeProofText(order.BankName) {
		reasons = append(reasons, "destination_bank_mismatch")
	}
	if normalizeProofDigits(input.DestinationAccount) == "" ||
		normalizeProofDigits(input.DestinationAccount) != normalizeProofDigits(order.BankAccountNumber) {
		reasons = append(reasons, "destination_account_mismatch")
	}
	// Bank receipts do not normally contain the website's internal order ID.
	// Check a detected reference when present, without inventing it in the client.
	if input.OrderID != "" && !strings.EqualFold(normalizePetroRunCode(input.OrderID), order.OrderNumber) {
		reasons = append(reasons, "order_id_mismatch")
	}
	if input.TransactionAt == "" || input.Timezone == "" || (input.Precision != "minute" && input.Precision != "second") {
		reasons = append(reasons, "transaction_timestamp_incomplete")
	} else if parsed, err := time.Parse(time.RFC3339Nano, input.TransactionAt); err != nil {
		reasons = append(reasons, "transaction_timestamp_invalid")
	} else {
		_, timestampOffset := parsed.Zone()
		parsed = parsed.UTC()
		transactionAt = &parsed
		if location, err := time.LoadLocation(input.Timezone); err != nil {
			reasons = append(reasons, "transaction_timezone_invalid")
		} else {
			_, timezoneOffset := parsed.In(location).Zone()
			if timestampOffset != timezoneOffset {
				reasons = append(reasons, "transaction_timezone_mismatch")
			}
		}
		if input.Precision == "minute" && (parsed.Second() != 0 || parsed.Nanosecond() != 0) {
			reasons = append(reasons, "transaction_precision_mismatch")
		}
		if parsed.Before(order.CreatedAt) || parsed.After(order.ExpiresAt) || parsed.After(now) {
			reasons = append(reasons, "transaction_outside_payment_window")
		}
	}
	if !now.Before(order.ExpiresAt) {
		reasons = append(reasons, "late_submission")
	}
	return input, transactionAt, reasons, nil
}

func normalizeProofText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}

func normalizeProofDigits(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, value)
}

func (s *petroRunService) ListPaymentProofs(ctx context.Context, status string, page, pageSize int) ([]domain.PetroRunPaymentProofReviewItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	records, total, err := s.repository.ListPaymentProofs(ctx, status, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.PetroRunPaymentProofReviewItem, len(records))
	for index, record := range records {
		proof := toPaymentProofResponse(record.Proof)
		proof.OrderID = record.Order.OrderNumber
		items[index] = domain.PetroRunPaymentProofReviewItem{
			Proof:        proof,
			OrderNumber:  record.Order.OrderNumber,
			Registrant:   record.Order.RegistrantName,
			CreatedAt:    record.Order.CreatedAt.UTC(),
			ExpiresAt:    record.Order.ExpiresAt.UTC(),
			PayableTotal: record.Order.PayableTotalIDR,
			Reference:    record.Proof.OCRReference,
			OCROrderID:   record.Proof.OCROrderID,
			RawText:      record.Proof.OCRRawText,
			FileURL:      fmt.Sprintf("/api/v1/petrorun/admin/payment-proofs/%s/file", record.Proof.ID),
		}
	}
	return items, total, nil
}

func (s *petroRunService) GetPaymentProofFile(ctx context.Context, proofID uuid.UUID) (io.ReadCloser, string, error) {
	record, err := s.repository.GetPaymentProof(ctx, proofID)
	if err != nil {
		return nil, "", err
	}
	if s.proofStore == nil {
		return nil, "", domain.ErrPetroRunUnavailable
	}
	file, err := s.proofStore.Open(record.Proof.FileKey)
	if err != nil {
		return nil, "", fmt.Errorf("open PetroRun payment proof: %w", err)
	}
	return file, record.Proof.MIMEType, nil
}

func (s *petroRunService) ReviewPaymentProof(ctx context.Context, proofID uuid.UUID, approve bool, reviewerID uuid.UUID, note string) (domain.PetroRunPaymentProofResponse, error) {
	note = strings.TrimSpace(note)
	if !approve && note == "" {
		return domain.PetroRunPaymentProofResponse{}, myerror.NewFieldValidation("note", "Review note", "is required when rejecting a payment proof")
	}
	if len(note) > 2000 {
		return domain.PetroRunPaymentProofResponse{}, myerror.NewFieldValidation("note", "Review note", "must not exceed 2000 characters")
	}
	if reviewerID == uuid.Nil {
		return domain.PetroRunPaymentProofResponse{}, domain.ErrUnauthenticated
	}
	for attempt := 0; attempt < maxApproveAttempts; attempt++ {
		record, err := s.repository.ReviewPaymentProof(ctx, proofID, approve, reviewerID, note, s.now().UTC(), s.newTicket)
		if !errors.Is(err, domain.ErrPetroRunConflict) {
			if err != nil {
				return domain.PetroRunPaymentProofResponse{}, err
			}
			result := toPaymentProofResponse(record.Proof)
			result.OrderID = record.Order.OrderNumber
			return result, nil
		}
	}
	return domain.PetroRunPaymentProofResponse{}, domain.ErrPetroRunConflict
}

func toPaymentProofResponse(proof entity.PetroRunPaymentProof) domain.PetroRunPaymentProofResponse {
	reasons := make([]string, 0)
	if proof.OCRMismatchReasons != "" {
		reasons = strings.Split(proof.OCRMismatchReasons, "\n")
	}
	return domain.PetroRunPaymentProofResponse{
		ID:                 proof.ID.String(),
		Attempt:            proof.Attempt,
		Status:             proof.Status,
		MIMEType:           proof.MIMEType,
		UploadedAt:         proof.CreatedAt.UTC(),
		SenderName:         proof.OCRSenderName,
		RecipientName:      proof.OCRRecipientName,
		TransactionStatus:  proof.OCRTransactionStatus,
		TransactionAt:      proof.OCRTransactionAt,
		ExtractedAmount:    proof.OCRExtractedAmount,
		DestinationBank:    proof.OCRDestinationBank,
		DestinationAccount: proof.OCRDestinationAccount,
		OCRClassification:  proof.OCRClassification,
		OCRConfidence:      proof.OCRConfidence,
		OCRTimezone:        proof.OCRTimezone,
		OCRPrecision:       proof.OCRPrecision,
		OCRSource:          proof.OCRSource,
		MismatchReasons:    reasons,
		RejectionReason:    proof.RejectionReason,
		LateSubmission:     proof.LateSubmission,
	}
}

// Dikirim ke repository.ApproveOrder dan dipanggil sekali per peserta
// di dalam transaksi approve.
func (s *petroRunService) newTicket(participant entity.PetroRunParticipant, ticketNumber string) (entity.PetroRunTicket, error) {
	// ID dibuat di Go, bukan default DB, karena hash token butuh ID sebelum insert.
	id := uuid.New()
	return entity.PetroRunTicket{
		ID:            id,
		ParticipantID: participant.ID,
		TicketNumber:  ticketNumber,
		TokenHash:     hashPetroRunToken(s.ticketToken(id)),
		Status:        "active",
	}, nil
}

// Token QR = HMAC(secret, "ticket:" + ticket ID). Selalu sama untuk tiket
// yang sama, jadi refresh halaman tidak mengubah QR. Prefix "ticket:"
// membedakannya dari access token order.
func (s *petroRunService) ticketToken(ticketID uuid.UUID) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.AccessSecret))
	mac.Write([]byte("ticket:"))
	mac.Write(ticketID[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func hashPetroRunToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func isValidPetroRunTicketNumber(ticketNumber string) bool {
	const prefix = "PR27-TKT-"
	if !strings.HasPrefix(ticketNumber, prefix) {
		// Tiket yang sudah terbit sebelum perubahan format tetap bisa dicari.
		const legacyPrefix = "PR27-RUN-"
		if !strings.HasPrefix(ticketNumber, legacyPrefix) || len(ticketNumber) != len("PR27-RUN-XXXX-XXXX") {
			return false
		}
		for i, char := range ticketNumber[len(legacyPrefix):] {
			if i == 4 {
				if char != '-' {
					return false
				}
				continue
			}
			if !strings.ContainsRune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789", char) {
				return false
			}
		}
		return true
	}
	if len(ticketNumber)-len(prefix) < 3 {
		return false
	}
	for _, digit := range ticketNumber[len(prefix):] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// Peserta sering mengetik ulang nomor dengan huruf kecil atau spasi.
func normalizePetroRunCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func (s *petroRunService) toTicketsResponse(order entity.PetroRunOrder) domain.PetroRunOrderTicketsResponse {
	status := order.Status
	// Order kedaluwarsa baru diubah statusnya saat ada order baru dibuat
	// (expireOverdueUpdate), jadi cek waktunya di sini juga.
	if status == entity.PetroRunWaitingPayment && !s.now().Before(order.ExpiresAt) {
		status = entity.PetroRunExpired
	}

	tickets := make([]domain.PetroRunTicketResponse, 0, len(order.Participants))
	if status == entity.PetroRunApproved {
		for _, participant := range order.Participants {
			if participant.Ticket != nil {
				tickets = append(tickets, s.toTicketResponse(participant))
			}
		}
	}

	return domain.PetroRunOrderTicketsResponse{
		OrderID: order.OrderNumber,
		Status:  string(status),
		Tickets: tickets,
	}
}

// Participant harus di-preload bersama Ticket-nya.
func (s *petroRunService) toTicketResponse(participant entity.PetroRunParticipant) domain.PetroRunTicketResponse {
	return domain.PetroRunTicketResponse{
		TicketNumber:    participant.Ticket.TicketNumber,
		ParticipantName: participant.Name,
		JerseySize:      participant.JerseySize,
		PaceTarget:      participant.RunningPace,
		Status:          participant.Ticket.Status,
		QRPayload:       petroRunQRPrefix + s.ticketToken(participant.Ticket.ID),
	}
}
