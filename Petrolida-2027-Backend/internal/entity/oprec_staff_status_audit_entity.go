package entity

import "github.com/google/uuid"

type OprecStaffStatusAudit struct {
	ID                            uuid.UUID        `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OprecStaffID                  uuid.UUID        `gorm:"type:uuid;not null;index" json:"oprec_staff_id"`
	ActorID                       uuid.UUID        `gorm:"type:uuid;not null;index" json:"actor_id"`
	ActorRole                     UserRole         `gorm:"type:varchar(20);not null;default:'superadmin'" json:"actor_role"`
	ActorDivisionID               *uuid.UUID       `gorm:"type:uuid;index" json:"actor_division_id,omitempty"`
	PreviousStatus                OprecStaffStatus `gorm:"type:varchar(20);not null" json:"previous_status"`
	NewStatus                     OprecStaffStatus `gorm:"type:varchar(20);not null" json:"new_status"`
	RejectionTarget               string           `gorm:"type:varchar(10);not null;default:''" json:"rejection_target,omitempty"`
	PreviousPrimaryRejected       bool             `gorm:"not null;default:false" json:"previous_primary_rejected"`
	NewPrimaryRejected            bool             `gorm:"not null;default:false" json:"new_primary_rejected"`
	PreviousSecondaryRejected     bool             `gorm:"not null;default:false" json:"previous_secondary_rejected"`
	NewSecondaryRejected          bool             `gorm:"not null;default:false" json:"new_secondary_rejected"`
	PreviousDivisionID            *uuid.UUID       `gorm:"type:uuid" json:"previous_division_id,omitempty"`
	NewDivisionID                 *uuid.UUID       `gorm:"type:uuid" json:"new_division_id,omitempty"`
	PreviousAcceptedSubdivisionID *uuid.UUID       `gorm:"type:uuid" json:"previous_accepted_subdivision_id,omitempty"`
	NewAcceptedSubdivisionID      *uuid.UUID       `gorm:"type:uuid" json:"new_accepted_subdivision_id,omitempty"`
	PreviousCurrentSubdivisionID  *uuid.UUID       `gorm:"type:uuid" json:"previous_current_subdivision_id,omitempty"`
	NewCurrentSubdivisionID       *uuid.UUID       `gorm:"type:uuid" json:"new_current_subdivision_id,omitempty"`

	Timestamp Timestamp `gorm:"embedded"`

	OprecStaff                  *OprecStaff `gorm:"foreignKey:OprecStaffID" json:"oprec_staff,omitempty"`
	Actor                       *User       `gorm:"foreignKey:ActorID" json:"actor,omitempty"`
	ActorDivision               *Divisi     `gorm:"foreignKey:ActorDivisionID" json:"actor_division,omitempty"`
	PreviousDivision            *Divisi     `gorm:"foreignKey:PreviousDivisionID;references:ID" json:"previous_division,omitempty"`
	NewDivision                 *Divisi     `gorm:"foreignKey:NewDivisionID;references:ID" json:"new_division,omitempty"`
	PreviousAcceptedSubdivision *Subdivisi  `gorm:"foreignKey:PreviousAcceptedSubdivisionID;references:ID" json:"previous_accepted_subdivision,omitempty"`
	NewAcceptedSubdivision      *Subdivisi  `gorm:"foreignKey:NewAcceptedSubdivisionID;references:ID" json:"new_accepted_subdivision,omitempty"`
	PreviousCurrentSubdivision  *Subdivisi  `gorm:"foreignKey:PreviousCurrentSubdivisionID;references:ID" json:"previous_current_subdivision,omitempty"`
	NewCurrentSubdivision       *Subdivisi  `gorm:"foreignKey:NewCurrentSubdivisionID;references:ID" json:"new_current_subdivision,omitempty"`
}

func (OprecStaffStatusAudit) TableName() string {
	return "oprec_staff_status_audits"
}
