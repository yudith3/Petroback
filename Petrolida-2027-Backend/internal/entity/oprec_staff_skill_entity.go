package entity

import "github.com/google/uuid"

type OprecStaffSkillLevel string

const (
	OprecStaffSkillLevelNone     OprecStaffSkillLevel = "no_proficiency"
	OprecStaffSkillLevelPoor     OprecStaffSkillLevel = "poor"
	OprecStaffSkillLevelDecent   OprecStaffSkillLevel = "decent"
	OprecStaffSkillLevelGood     OprecStaffSkillLevel = "good"
	OprecStaffSkillLevelVeryGood OprecStaffSkillLevel = "very_good"
)

func (level OprecStaffSkillLevel) IsValid() bool {
	switch level {
	case OprecStaffSkillLevelNone, OprecStaffSkillLevelPoor, OprecStaffSkillLevelDecent,
		OprecStaffSkillLevelGood, OprecStaffSkillLevelVeryGood:
		return true
	default:
		return false
	}
}

type OprecStaffSkill struct {
	ID                 uuid.UUID            `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OprecStaffID       uuid.UUID            `gorm:"type:uuid;not null;uniqueIndex" json:"oprec_staff_id"`
	EnglishProficiency OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"english_proficiency"`
	CommunicationSkill OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"communication_skill"`
	CreativityInEvent  OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"creativity_in_event"`
	FastResponse       OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"fast_response"`
	GraphicDesign      OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"graphic_design"`
	Photography        OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"photography"`
	VideoEditing       OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"video_editing"`
	WebDesign          OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"web_design"`
	OperatingZoom      OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"operating_zoom"`
	SocialMediaTrends  OprecStaffSkillLevel `gorm:"type:varchar(20);not null" json:"social_media_trends"`
	DesignApplications string               `gorm:"type:text;not null" json:"design_applications"`

	Timestamp  Timestamp   `gorm:"embedded"`
	OprecStaff *OprecStaff `gorm:"foreignKey:OprecStaffID" json:"oprec_staff,omitempty"`
}

func (OprecStaffSkill) TableName() string {
	return "oprec_staff_skills"
}
