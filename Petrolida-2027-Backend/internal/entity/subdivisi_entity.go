package entity

import "github.com/google/uuid"

type Subdivisi struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	DivisiID  uuid.UUID `gorm:"type:uuid;not null" json:"divisi_id"`
	Nama      string    `gorm:"type:varchar(100);not null;unique" json:"name"`
	Deskripsi *string   `gorm:"type:text" json:"deskripsi"`
	IsActive  bool      `gorm:"type:boolean;not null;default:true" json:"is_active"`

	Timestamp Timestamp `gorm:"embedded"`

	Divisi  *Divisi            `gorm:"foreignKey:DivisiID" json:"divisi,omitempty"`
	Choices []OprecStaffChoice `gorm:"foreignKey:SubdivisiID" json:"choices,omitempty"`
}

func (Subdivisi) TableName() string {
	return "subdivisi"
}
