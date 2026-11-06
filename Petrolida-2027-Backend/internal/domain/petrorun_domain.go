package domain

import (
	"time"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
)

var (
	ErrPetroRunOrderNotFound      = myerror.New(myerror.TypeNotFound, "PETRORUN_ORDER_NOT_FOUND", "petro run order not found")
	ErrPetroRunConflict           = myerror.New(myerror.TypeConflict, "PETRORUN_ORDER_CONFLICT", "order conflicts with an existing order")
	ErrPetroRunExhausted          = myerror.New(myerror.TypeConflict, "PETRORUN_ORDER_EXHAUSTED", "payment codes are temporarily full, please retry again")
	ErrPetroRunUnavailable        = myerror.New(myerror.TypeInternal, "PETRORUN_UNAVAILABLE", "petro run registration is not configured")
	ErrPetroRunRegistrationClosed = myerror.New(myerror.TypeConflict, "PETRORUN_REGISTRATION_CLOSED", "PetroRun registration is outside the registration period")
	ErrPetroRunEarlyBirdExhausted = myerror.New(myerror.TypeConflict, "PETRORUN_EARLY_BIRD_EXHAUSTED", "The expired early bird reservation has been used by another order. Resolve the payment with the organizer before approval; this invoice has not been repriced.")

	// Token salah sengaja memakai kode yang sama dengan order tidak ditemukan,
	// supaya orang luar tidak bisa menebak order mana yang ada.
	ErrPetroRunInvalidAccess  = myerror.New(myerror.TypeNotFound, "PETRORUN_ORDER_NOT_FOUND", "petro run order not found")
	ErrPetroRunTicketNotFound = myerror.New(myerror.TypeNotFound, "PETRORUN_TICKET_NOT_FOUND", "ticket not found")
	ErrPetroRunNotApprovable  = myerror.New(myerror.TypeConflict, "PETRORUN_NOT_APPROVABLE", "order cannot be approved in its current status")
	ErrPetroRunProofNotFound  = myerror.New(myerror.TypeNotFound, "PETRORUN_PROOF_NOT_FOUND", "payment proof not found")
	ErrPetroRunOrderPaid      = myerror.New(myerror.TypeConflict, "PETRORUN_ORDER_ALREADY_APPROVED", "order has already been approved")
	ErrPetroRunProofReviewed  = myerror.New(myerror.TypeConflict, "PETRORUN_PROOF_ALREADY_REVIEWED", "payment proof has already been reviewed")
)

func PetroRunUniqueCode(baseTotal int64, code int) int64 {
	return (int64(code) - baseTotal%1000 + 1000) % 1000
}

type CreatePetroRunOrderInput struct {
	FullName                string                     `json:"full_name"`
	Email                   string                     `json:"email"`
	WhatsappNumber          string                     `json:"whatsapp_number"`
	RegistrantIsParticipant bool                       `json:"registrant_is_participant"`
	Participants            []PetroRunParticipantInput `json:"participants"`
	AcceptTerms             bool                       `json:"accept_terms"`
	AcceptPrivacy           bool                       `json:"accept_privacy"`
}

type PetroRunParticipantInput struct {
	FullName       string `json:"full_name"`
	JerseySize     string `json:"jersey_size"`
	PaceTarget     string `json:"pace_target"`
	MedicalHistory string `json:"medical_history"`
}

type PetroRunOrderResponse struct {
	OrderID      string          `json:"order_id"`
	Status       string          `json:"status"`
	Pricing      PetroRunPricing `json:"pricing"`
	PayableTotal int64           `json:"payable_total"`
	Bank         PetroRunBank    `json:"bank"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	ServerTime   time.Time       `json:"server_time"`
	AccessToken  string          `json:"access_token"`
}

type PetroRunPricing struct {
	Lines            []PetroRunPriceLine `json:"lines,omitempty"`
	BaseSubtotal     int64               `json:"base_subtotal"`
	JerseySurcharge  int64               `json:"jersey_surcharge"`
	AdminFee         int64               `json:"admin_fee"`
	UniqueCode       int                 `json:"unique_code"`
	UniqueAdjustment int64               `json:"unique_adjustment"`
}

type PetroRunBank struct {
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

// Data tiket untuk e-ticket, cek tiket, dan hasil scan QR. Jangan tambahkan
// email, kontak, atau medical history ke struct ini.
type PetroRunTicketResponse struct {
	TicketNumber    string `json:"ticket_number"`
	ParticipantName string `json:"participant_name"`
	JerseySize      string `json:"jersey_size"`
	PaceTarget      string `json:"pace_target"`
	Status          string `json:"status"`
	QRPayload       string `json:"qr_payload"`
}

type PetroRunOrderTicketsResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
	// Selalu [] (bukan null) selama order belum approved.
	Tickets []PetroRunTicketResponse `json:"tickets"`
}

type VerifyPetroRunTicketInput struct {
	QRPayload string `json:"qr_payload"`
}

type PetroRunOCRInput struct {
	Classification     string   `json:"classification"`
	RawText            string   `json:"raw_text"`
	SenderName         string   `json:"sender_name"`
	RecipientName      string   `json:"recipient_name"`
	TransactionStatus  string   `json:"transaction_status"`
	ExtractedAmount    *int64   `json:"extracted_amount"`
	DestinationAccount string   `json:"destination_account"`
	DestinationBank    string   `json:"destination_bank"`
	TransactionAt      string   `json:"transaction_at"`
	Timezone           string   `json:"timezone"`
	Precision          string   `json:"precision"`
	Confidence         *float64 `json:"confidence"`
	Reference          string   `json:"reference"`
	OrderID            string   `json:"order_id"`
}

type PetroRunPaymentProofResponse struct {
	ID                 string     `json:"id"`
	OrderID            string     `json:"order_id"`
	Attempt            int        `json:"attempt"`
	Status             string     `json:"status"`
	MIMEType           string     `json:"mime_type"`
	UploadedAt         time.Time  `json:"uploaded_at"`
	SenderName         string     `json:"sender_name,omitempty"`
	RecipientName      string     `json:"recipient_name,omitempty"`
	TransactionStatus  string     `json:"transaction_status"`
	TransactionAt      *time.Time `json:"transaction_at,omitempty"`
	ExtractedAmount    *int64     `json:"extracted_amount,omitempty"`
	DestinationBank    string     `json:"destination_bank,omitempty"`
	DestinationAccount string     `json:"destination_account,omitempty"`
	OCRClassification  string     `json:"ocr_classification"`
	OCRConfidence      *float64   `json:"ocr_confidence,omitempty"`
	OCRTimezone        string     `json:"ocr_timezone,omitempty"`
	OCRPrecision       string     `json:"ocr_precision,omitempty"`
	OCRSource          string     `json:"ocr_source"`
	MismatchReasons    []string   `json:"mismatch_reasons"`
	RejectionReason    string     `json:"rejection_reason,omitempty"`
	LateSubmission     bool       `json:"late_submission"`
}

type PetroRunPaymentProofReviewItem struct {
	Proof        PetroRunPaymentProofResponse `json:"proof"`
	OrderNumber  string                       `json:"order_number"`
	Registrant   string                       `json:"registrant"`
	CreatedAt    time.Time                    `json:"order_created_at"`
	ExpiresAt    time.Time                    `json:"order_expires_at"`
	PayableTotal int64                        `json:"payable_total"`
	Reference    string                       `json:"reference,omitempty"`
	OCROrderID   string                       `json:"ocr_order_id,omitempty"`
	RawText      string                       `json:"ocr_raw_text,omitempty"`
	FileURL      string                       `json:"file_url"`
}

type ReviewPetroRunPaymentProofInput struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}
