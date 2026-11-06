package entity

import (
	"github.com/google/uuid"
)

type OprecStaffChoice struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OprecStaffID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_oprec_staff_choices_staff_pilihan,priority:1" json:"oprec_staff_id"`
	Pilihan         int16     `gorm:"type:smallint;not null;uniqueIndex:idx_oprec_staff_choices_staff_pilihan,priority:2;check:chk_oprec_staff_choices_pilihan,pilihan IN (1, 2)" json:"pilihan"`
	AlasanBergabung string    `gorm:"type:text;not null" json:"alasan_bergabung"`
	PemahamanDivisi string    `gorm:"type:text;not null" json:"pemahaman_divisi"`
	IdeInovasi      string    `gorm:"type:text;not null" json:"ide_inovasi"`
	SubdivisiID     uuid.UUID `gorm:"type:uuid;not null;index" json:"subdivisi_id"`
	Pengalaman      *string   `gorm:"type:text" json:"pengalaman"`

	Timestamp Timestamp `gorm:"embedded"`

	OprecStaff *OprecStaff `gorm:"foreignKey:OprecStaffID" json:"oprec_staff,omitempty"`
	Subdivisi  *Subdivisi  `gorm:"foreignKey:SubdivisiID" json:"subdivisi,omitempty"`
}

func (OprecStaffChoice) TableName() string {
	return "oprec_staff_choices"
}
