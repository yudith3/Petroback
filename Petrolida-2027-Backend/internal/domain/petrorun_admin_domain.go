package domain

import (
	"time"

	"github.com/google/uuid"
)

type PetroRunAdminQuery struct {
	Search, Status, Sort, Direction                        string
	CreatedFrom, CreatedTo, TransactionFrom, TransactionTo *time.Time
	Page, PageSize                                         int
}

type PetroRunSummary struct {
	TotalOrders       int64 `json:"total_orders"`
	TotalParticipants int64 `json:"total_participants"`
	WaitingPayment    int64 `json:"waiting_payment"`
	PendingReview     int64 `json:"pending_review"`
	Approved          int64 `json:"approved"`
	Rejected          int64 `json:"rejected"`
	Expired           int64 `json:"expired"`
	ApprovedTotal     int64 `json:"approved_total"`
}

type PetroRunAdminParticipant struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	JerseySize     string    `json:"jersey_size"`
	PaceTarget     string    `json:"pace_target"`
	MedicalHistory string    `json:"medical_history"`
}

type PetroRunAdminProof struct {
	PetroRunPaymentProofResponse
	RawText    string     `json:"ocr_raw_text"`
	Reference  string     `json:"reference"`
	OCROrderID string     `json:"ocr_order_id"`
	FileURL    string     `json:"file_url"`
	ReviewerID *uuid.UUID `json:"reviewer_id"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	ReviewNote string     `json:"review_note"`
}

type PetroRunAdminAudit struct {
	ID         uuid.UUID  `json:"id"`
	ActorID    *uuid.UUID `json:"actor_id"`
	Action     string     `json:"action"`
	FromStatus string     `json:"from_status"`
	ToStatus   string     `json:"to_status"`
	Reason     string     `json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
}

type PetroRunAdminOrder struct {
	OrderID            string                        `json:"order_id"`
	Status             string                        `json:"status"`
	RegistrantName     string                        `json:"registrant_name"`
	RegistrantEmail    string                        `json:"registrant_email"`
	RegistrantPhone    string                        `json:"registrant_phone"`
	ParticipantCount   int                           `json:"participant_count"`
	Pricing            PetroRunPricing               `json:"pricing"`
	PayableTotal       int64                         `json:"payable_total"`
	Bank               PetroRunBank                  `json:"bank"`
	CreatedAt          time.Time                     `json:"created_at"`
	ExpiresAt          time.Time                     `json:"expires_at"`
	ServerTime         time.Time                     `json:"server_time"`
	VerificationSource string                        `json:"verification_source"`
	LatestProof        *PetroRunPaymentProofResponse `json:"latest_proof,omitempty"`
	Participants       []PetroRunAdminParticipant    `json:"participants,omitempty"`
	Proofs             []PetroRunAdminProof          `json:"proofs,omitempty"`
	Tickets            []PetroRunTicketResponse      `json:"tickets,omitempty"`
	Audit              []PetroRunAdminAudit          `json:"audit,omitempty"`
}
