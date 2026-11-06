package entity

import (
	"github.com/google/uuid"
)

type Divisi struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Nama      string    `gorm:"type:varchar(100);not null;unique" json:"name"`
	Deskripsi *string   `gorm:"type:text" json:"deskripsi"`
	IsActive  bool      `gorm:"type:boolean;not null;default:true" json:"is_active"`

	Timestamp Timestamp `gorm:"embedded"`

	Subdivisi []Subdivisi `gorm:"foreignKey:DivisiID" json:"subdivisi,omitempty"`
}

func (Divisi) TableName() string {
	return "divisi"
}
