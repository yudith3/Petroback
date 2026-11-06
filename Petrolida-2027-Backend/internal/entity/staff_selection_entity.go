package entity

import "time"

const StaffSelectionID = "staff-2027"

// StaffSelection is a permanent singleton lock, without soft deletion.
// The roster and registrant decisions commit in the same transaction as this lock.
type StaffSelection struct {
	ID             string `gorm:"type:varchar(30);primaryKey"`
	FinalizedAt    *time.Time
	RosterHash     string `gorm:"type:varchar(64);not null;default:''"`
	AcceptedCount  int64  `gorm:"not null;default:0"`
	RejectedCount  int64  `gorm:"not null;default:0"`
	UnmatchedCount int64  `gorm:"not null;default:0"`
}

func (StaffSelection) TableName() string { return "oprec_staff_selections" }
