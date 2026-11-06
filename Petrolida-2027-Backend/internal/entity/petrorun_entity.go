package entity

import (
	"time"

	"github.com/google/uuid"
)

type PetroRunOrderStatus string

const (
	PetroRunWaitingPayment PetroRunOrderStatus = "waiting_payment"
	PetroRunPendingReview  PetroRunOrderStatus = "pending_review"
	PetroRunApproved       PetroRunOrderStatus = "approved"
	PetroRunRejected       PetroRunOrderStatus = "rejected"
	PetroRunExpired        PetroRunOrderStatus = "expired"
)

type PetroRunOrder struct {
	ID                                                                                      uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderNumber                                                                             string                `gorm:"uniqueIndex;not null" json:"order_id"`
	RegistrantName                                                                          string                `gorm:"not null" json:"registrant_name"`
	RegistrantEmail                                                                         string                `gorm:"not null" json:"registrant_email"`
	RegistrantPhone                                                                         string                `gorm:"not null" json:"registrant_phone"`
	Status                                                                                  PetroRunOrderStatus   `gorm:"type:varchar(30);index;not null" json:"status"`
	BasePriceIDR, AdminFeeIDR, ParticipantSubtotalIDR, UniqueAdjustmentIDR, PayableTotalIDR int64                 `gorm:"not null" json:"-"`
	BankName, BankAccountNumber, BankAccountHolder                                          string                `gorm:"not null" json:"-"`
	ExpiresAt                                                                               time.Time             `json:"expires_at"`
	Participants                                                                            []PetroRunParticipant `gorm:"foreignKey:OrderID;references:ID" json:"participants,omitempty"`
	Timestamp
	IdempotencyKey *uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"-"`

	UniqueCode int `gorm:"not null" json:"unique_code"`
	// Zero defaults preserve invoices created before tiered pricing was added.
	BaseSubtotalIDR   int64 `gorm:"not null;default:0" json:"-"`
	EarlyBirdQuantity int   `gorm:"not null;default:0" json:"-"`
	EarlyBirdQuota    int   `gorm:"not null;default:0" json:"-"`
	EarlyBirdReserved bool  `gorm:"not null;default:false" json:"-"`
}

func (PetroRunOrder) TableName() string {
	return "petro_run_orders"
}

type PetroRunParticipant struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrderID        uuid.UUID       `gorm:"type:uuid;not null;index"`
	Name           string          `gorm:"not null" json:"name"`
	BirthDate      time.Time       `gorm:"not null" json:"birth_date"`
	Email          string          `gorm:"not null" json:"email"`
	Contact        string          `gorm:"not null" json:"contact"`
	JerseySize     string          `gorm:"not null" json:"jersey_size"`
	MedicalHistory string          `gorm:"type:text;not null" json:"medical_history"`
	RunningPace    string          `gorm:"not null" json:"running_pace"`
	Ticket         *PetroRunTicket `gorm:"foreignKey:ParticipantID;references:ID" json:"ticket,omitempty"`
	BasePriceIDR   int64           `gorm:"not null;default:0" json:"base_price_idr"`
	PriceTier      string          `gorm:"not null;default:''" json:"price_tier"`
}

func (PetroRunParticipant) TableName() string {
	return "petro_run_participants"
}

type PetroRunPaymentProof struct {
	ID                    uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrderID               uuid.UUID `gorm:"type:uuid;not null;index"`
	Attempt               int       `gorm:"not null"`
	FileKey               string    `gorm:"not null"`
	FileSHA256            string    `gorm:"type:char(64);not null;index"`
	MIMEType              string    `gorm:"not null"`
	Status                string    `gorm:"type:varchar(32);not null;index"`
	RejectionReason       string    `gorm:"type:text;not null;default:''"`
	OCRSource             string    `gorm:"type:varchar(32);not null;default:'browser_client'"`
	OCRClassification     string    `gorm:"type:varchar(32);not null;default:'unknown'"`
	OCRRawText            string    `gorm:"type:text"`
	OCRSenderName         string    `gorm:"type:varchar(200)"`
	OCRRecipientName      string    `gorm:"type:varchar(200)"`
	OCRTransactionStatus  string    `gorm:"type:varchar(16);not null;default:'unknown'"`
	OCRMismatchReasons    string    `gorm:"type:text"`
	OCRExtractedAmount    *int64
	OCRDestinationAccount string
	OCRDestinationBank    string
	OCRTransactionAt      *time.Time
	OCRTimezone           string
	OCRPrecision          string
	OCRConfidence         *float64
	OCRReference          string     `gorm:"type:text"`
	OCROrderID            string     `gorm:"type:varchar(100)"`
	ReviewerID            *uuid.UUID `gorm:"type:uuid"`
	ReviewedAt            *time.Time
	ReviewNote            string        `gorm:"type:text"`
	LateSubmission        bool          `gorm:"not null;default:false"`
	Order                 PetroRunOrder `gorm:"foreignKey:OrderID;references:ID" json:"-"`
	Timestamp
}

func (PetroRunPaymentProof) TableName() string {
	return "petro_run_payment_proofs"
}

type PetroRunTicket struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ParticipantID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	TokenHash     string    `gorm:"uniqueIndex;not null"`
	TicketNumber  string    `gorm:"uniqueIndex;not null"`
	Status        string    `gorm:"not null;default:'active'"`
	Timestamp
}

func (PetroRunTicket) TableName() string {
	return "petro_run_tickets"
}

type PetroRunAuditLog struct {
	ID                                   uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrderID                              uuid.UUID  `gorm:"type:uuid;index;not null"`
	ActorID                              *uuid.UUID `gorm:"type:uuid"`
	Action, FromStatus, ToStatus, Reason string     `gorm:"not null"`
	Timestamp
}

func (PetroRunAuditLog) TableName() string {
	return "petro_run_audit_logs"
}
