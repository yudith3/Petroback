package entity

import (
	"github.com/google/uuid"
)

type Fakultas struct {
	ID   uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Nama string    `gorm:"type:varchar(150);not null;unique" json:"name"`

	Timestamp Timestamp `gorm:"embedded"`

	Departemen []Departemen `gorm:"foreignKey:FakultasID" json:"departemen,omitempty"`
}

func (Fakultas) TableName() string {
	return "fakultas"
}
