package domain

import (
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"time"
)

var ErrAnnouncementClosed = myerror.New(myerror.TypeAuthorization, "ANNOUNCEMENT_CLOSED", "announcement is not open")
var ErrAnnouncementPending = myerror.New(myerror.TypeConflict, "ANNOUNCEMENT_PENDING", "selection result is not finalized")
var ErrAnnouncementOnHold = myerror.New(myerror.TypeConflict, "ANNOUNCEMENT_ON_HOLD", "your administration result is on hold pending confirmation")

type AnnouncementInput struct {
	NRP string `json:"nrp"`
}
type AnnouncementResult struct {
	Name        string           `json:"name"`
	NRP         string           `json:"nrp"`
	Status      string           `json:"status"`
	Subdivision *MasterReference `json:"subdivision,omitempty"`
	ActionURL   string           `json:"action_url,omitempty"`
}
type AnnouncementStageConfig struct {
	Open           bool       `json:"open"`
	AnnouncementAt *time.Time `json:"announcement_at"`
}
type AnnouncementConfig struct {
	Staff      AnnouncementStageConfig `json:"staff"`
	Interview  AnnouncementStageConfig `json:"interview"`
	ServerTime time.Time               `json:"server_time"`
}
