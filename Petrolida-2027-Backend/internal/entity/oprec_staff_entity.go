package entity

import (
	"github.com/google/uuid"
)

type OprecStaffStatus string

const (
	OprecStaffStatusPending            OprecStaffStatus = "pending"
	OprecStaffStatusScreeningPrimary   OprecStaffStatus = "screening_primary"
	OprecStaffStatusScreeningSecondary OprecStaffStatus = "screening_secondary"
	OprecStaffStatusInterview          OprecStaffStatus = "interview"
	OprecStaffStatusAccepted           OprecStaffStatus = "accepted"
	OprecStaffStatusRejected           OprecStaffStatus = "rejected"
)

// Kolom status disimpan sebagai varchar, bukan ENUM Postgres, supaya migration
// jadi logic status merupakan tanggung jawab service layer lewat IsValid.
func (s OprecStaffStatus) IsValid() bool {
	switch s {
	case OprecStaffStatusPending,
		OprecStaffStatusScreeningPrimary,
		OprecStaffStatusScreeningSecondary,
		OprecStaffStatusInterview,
		OprecStaffStatusAccepted,
		OprecStaffStatusRejected:
		return true
	default:
		return false
	}
}

func (s OprecStaffStatus) String() string {
	return string(s)
}

type OprecStaff struct {
	ID                      uuid.UUID        `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SubmissionKey           *uuid.UUID       `gorm:"type:uuid;uniqueIndex" json:"-"`
	SubmissionFingerprint   *string          `gorm:"type:varchar(64)" json:"-"`
	Nama                    string           `gorm:"type:varchar(150);not null" json:"name"`
	Email                   string           `gorm:"type:varchar(255);not null;unique" json:"email"`
	NRP                     string           `gorm:"type:varchar(30);not null;unique" json:"nrp"`
	NomorTelepon            string           `gorm:"type:varchar(30);not null" json:"nomor_telepon"`
	LineID                  *string          `gorm:"type:varchar(100)" json:"line_id"`
	FakultasID              uuid.UUID        `gorm:"type:uuid;not null;index" json:"fakultas_id"`
	DepartemenID            uuid.UUID        `gorm:"type:uuid;not null;index" json:"departemen_id"`
	Angkatan                int16            `gorm:"type:smallint;not null" json:"angkatan"`
	TentangPetrolida        string           `gorm:"type:text;not null" json:"tentang_petrolida"`
	WillingTransferDivision bool             `gorm:"not null;default:false" json:"willing_transfer_division"`
	Status                  OprecStaffStatus `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	PrimaryRejected         bool             `gorm:"not null;default:false" json:"primary_rejected"`
	SecondaryRejected       bool             `gorm:"not null;default:false" json:"secondary_rejected"`
	AcceptedDivisiID        *uuid.UUID       `gorm:"column:accepted_divisi_id;type:uuid" json:"accepted_divisi_id"`
	CurrentSubdivisionID    *uuid.UUID       `gorm:"type:uuid" json:"current_subdivision_id"`

	Timestamp Timestamp `gorm:"embedded"`

	Fakultas           *Fakultas                        `gorm:"foreignKey:FakultasID" json:"fakultas,omitempty"`
	Departemen         *Departemen                      `gorm:"foreignKey:DepartemenID" json:"departemen,omitempty"`
	AcceptedDivisi     *Subdivisi                       `gorm:"foreignKey:AcceptedDivisiID;references:ID" json:"accepted_divisi,omitempty"`
	CurrentSubdivision *Subdivisi                       `gorm:"foreignKey:CurrentSubdivisionID;references:ID" json:"current_subdivision,omitempty"`
	Choices            []OprecStaffChoice               `gorm:"foreignKey:OprecStaffID" json:"choices,omitempty"`
	Documents          []OprecStaffDocument             `gorm:"foreignKey:OprecStaffID" json:"documents,omitempty"`
	Skills             *OprecStaffSkill                 `gorm:"foreignKey:OprecStaffID" json:"skills,omitempty"`
	AdditionalInfo     *OprecStaffAdditionalInformation `gorm:"foreignKey:OprecStaffID" json:"additional_info,omitempty"`
}

func (OprecStaff) TableName() string {
	return "oprec_staff"
}
