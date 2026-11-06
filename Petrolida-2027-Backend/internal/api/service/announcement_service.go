package service

import (
	"context"
	"errors"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"regexp"
	"strings"
	"time"
)

type AnnouncementService interface {
	Config(time.Time) domain.AnnouncementConfig
	Lookup(context.Context, string, domain.AnnouncementInput, time.Time) (domain.AnnouncementResult, error)
}
type announcementService struct {
	repository repository.AnnouncementRepository
	config     config.OprecConfig
}

func NewAnnouncement(repo repository.AnnouncementRepository, cfg config.OprecConfig) AnnouncementService {
	return &announcementService{repo, cfg}
}
func stageConfig(at, now time.Time) domain.AnnouncementStageConfig {
	if at.IsZero() {
		return domain.AnnouncementStageConfig{}
	}
	return domain.AnnouncementStageConfig{Open: !now.Before(at), AnnouncementAt: &at}
}
func (s *announcementService) Config(now time.Time) domain.AnnouncementConfig {
	return domain.AnnouncementConfig{Staff: stageConfig(s.config.AnnouncementAt, now), Interview: stageConfig(s.config.InterviewAnnouncementAt, now), ServerTime: now}
}

// The published roster explicitly retains this nonstandard Student ID.
var announcementNRPPattern = regexp.MustCompile(`^([0-9]{10}|351518190408001)$`)

func (s *announcementService) Lookup(ctx context.Context, stage string, input domain.AnnouncementInput, now time.Time) (domain.AnnouncementResult, error) {
	empty := domain.AnnouncementResult{}
	if stage != "staff" && stage != "interview" {
		return empty, domain.ErrRouteNotFound
	}
	nrp := strings.TrimSpace(input.NRP)
	if !announcementNRPPattern.MatchString(nrp) {
		return empty, myerror.NewValidation([]myerror.FieldError{{Field: "nrp", Label: "Student ID", Message: "must contain 10 digits or be the retained 15-digit Student ID"}})
	}
	at := s.config.AnnouncementAt
	if stage == "interview" {
		at = s.config.InterviewAnnouncementAt
	}
	if !stageConfig(at, now).Open {
		return empty, domain.ErrAnnouncementClosed
	}
	if stage == "interview" {
		return s.lookupInterview(ctx, nrp)
	}
	return s.lookupStaff(ctx, nrp)
}

func (s *announcementService) lookupStaff(ctx context.Context, nrp string) (domain.AnnouncementResult, error) {
	applicant, err := s.repository.GetByNRP(ctx, nrp)
	if err != nil {
		return domain.AnnouncementResult{}, err
	}
	result := domain.AnnouncementResult{Name: applicant.Nama, NRP: applicant.NRP}
	switch applicant.Status {
	case entity.OprecStaffStatusAccepted:
		if applicant.AcceptedDivisi == nil {
			return domain.AnnouncementResult{}, domain.ErrAnnouncementPending
		}
		result.Status = "accepted"
		result.Subdivision = &domain.MasterReference{ID: applicant.AcceptedDivisi.ID, Name: applicant.AcceptedDivisi.Nama}
		result.ActionURL = s.config.StaffGroupURL
	case entity.OprecStaffStatusRejected:
		result.Status = "rejected"
	default:
		return domain.AnnouncementResult{}, domain.ErrAnnouncementPending
	}
	return result, nil
}

func (s *announcementService) lookupInterview(ctx context.Context, nrp string) (domain.AnnouncementResult, error) {
	snapshot, err := s.repository.GetInterviewResult(ctx, nrp)
	if err == nil {
		if snapshot.Status == "on_hold" {
			return domain.AnnouncementResult{}, domain.ErrAnnouncementOnHold
		}
		if snapshot.Status != "accepted" {
			return domain.AnnouncementResult{}, domain.ErrAnnouncementPending
		}
		if snapshot.NRP != "351518190408001" && (snapshot.Name == "" || snapshot.Subdivision == nil) {
			return domain.AnnouncementResult{}, domain.ErrAnnouncementPending
		}
		// The interview subdivision stays internal: the published result only
		// confirms that the applicant passed administration.
		return domain.AnnouncementResult{
			Name: snapshot.Name, NRP: snapshot.NRP, Status: "accepted", ActionURL: s.config.InterviewScheduleURL,
		}, nil
	}
	if !errors.Is(err, domain.ErrRegistrantNotFound) {
		return domain.AnnouncementResult{}, err
	}
	// The approved roster is complete: a known registrant outside it did not
	// pass administration, regardless of their current admin selection status.
	applicant, err := s.repository.GetByNRP(ctx, nrp)
	if err != nil {
		return domain.AnnouncementResult{}, err
	}
	return domain.AnnouncementResult{Name: applicant.Nama, NRP: applicant.NRP, Status: "rejected"}, nil
}
