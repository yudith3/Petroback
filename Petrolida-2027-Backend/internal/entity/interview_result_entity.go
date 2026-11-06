package entity

import "github.com/google/uuid"

// InterviewResult freezes the published administration result independently
// of later screening, interview, and final staff decisions.
type InterviewResult struct {
	NRP           string     `gorm:"type:varchar(30);primaryKey" json:"nrp"`
	Name          string     `gorm:"type:varchar(150);not null" json:"name"`
	Status        string     `gorm:"type:varchar(20);not null;check:interview_result_status,status IN ('accepted', 'on_hold')" json:"status"`
	SubdivisionID *uuid.UUID `gorm:"type:uuid" json:"subdivision_id,omitempty"`
	Subdivision   *Subdivisi `gorm:"foreignKey:SubdivisionID" json:"subdivision,omitempty"`
	Timestamp     Timestamp  `gorm:"embedded"`
}

func (InterviewResult) TableName() string { return "oprec_interview_results" }
