package entity

import (
	"github.com/google/uuid"
)

type Departemen struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FakultasID uuid.UUID `gorm:"type:uuid;not null" json:"fakultas_id"`
	Nama       string    `gorm:"type:varchar(150);not null" json:"name"`

	Timestamp Timestamp `gorm:"embedded"`

	Fakultas *Fakultas `gorm:"foreignKey:FakultasID" json:"fakultas,omitempty"`
}

func (Departemen) TableName() string {
	return "departemen"
}
