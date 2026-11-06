package entity

import (
	"github.com/google/uuid"
)

type OprecStaffDocument struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OprecStaffID uuid.UUID `gorm:"type:uuid;not null;index" json:"oprec_staff_id"`
	Tipe         string    `gorm:"type:varchar(30);not null" json:"tipe"`
	URL          string    `gorm:"type:text;not null" json:"url"`
	OriginalName *string   `gorm:"type:varchar(255)" json:"original_name,omitempty"`
	MimeType     *string   `gorm:"type:varchar(100)" json:"mime_type,omitempty"`
	SizeBytes    *int64    `gorm:"type:bigint" json:"size_bytes,omitempty"`

	Timestamp Timestamp `gorm:"embedded"`

	OprecStaff *OprecStaff `gorm:"foreignKey:OprecStaffID" json:"oprec_staff,omitempty"`
}

func (OprecStaffDocument) TableName() string {
	return "oprec_staff_documents"
}
