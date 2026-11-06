package entity

import "github.com/google/uuid"

type OprecStaffAdditionalInformation struct {
	ID                  uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OprecStaffID        uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"oprec_staff_id"`
	HasCarInSurabaya    bool      `gorm:"not null" json:"has_car_in_surabaya"`
	HasSponsorRelation  bool      `gorm:"not null" json:"has_sponsor_relation"`
	HasVendorInSurabaya bool      `gorm:"not null" json:"has_vendor_in_surabaya"`
	HasCamera           bool      `gorm:"not null" json:"has_camera"`
	HasLaptop           bool      `gorm:"not null" json:"has_laptop"`
	HasTablet           bool      `gorm:"not null" json:"has_tablet"`
	ConnectedCompany    string    `gorm:"type:text;not null" json:"connected_company"`

	Timestamp  Timestamp   `gorm:"embedded"`
	OprecStaff *OprecStaff `gorm:"foreignKey:OprecStaffID" json:"oprec_staff,omitempty"`
}

func (OprecStaffAdditionalInformation) TableName() string {
	return "oprec_staff_additional_information"
}
