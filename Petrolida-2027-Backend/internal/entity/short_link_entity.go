package entity

import "github.com/google/uuid"

type ShortLink struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Slug           string    `gorm:"type:varchar(48);not null;uniqueIndex" json:"slug"`
	DestinationURL string    `gorm:"type:text;not null" json:"destination_url"`
	ClickCount     int64     `gorm:"not null;default:0" json:"click_count"`
	CreatedBy      uuid.UUID `gorm:"type:uuid;not null;index" json:"created_by"`

	Timestamp Timestamp `gorm:"embedded"`
}

func (ShortLink) TableName() string {
	return "short_links"
}
