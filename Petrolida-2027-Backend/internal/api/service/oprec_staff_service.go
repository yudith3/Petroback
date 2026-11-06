package service

import (
	"context"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/export"
	portfoliostorage "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/storage"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/utils"
	"github.com/google/uuid"
)

// Alur pendaftaran ada di oprec_registration.go, pembuatan entity di oprec_builders.go,
// dan pemetaan ke response di oprec_mapper.go.

type OprecStaffService interface {
	ConfirmRegistration(ctx context.Context, submissionKey string) (domain.RegistrationConfirmation, error)
	Config(now time.Time) domain.OprecConfigResponse
	Register(input domain.RegistrationInput, portfolio *multipart.FileHeader) (domain.RegistrationResult, error)
	List(admin domain.AdminProfile, params domain.RegistrantListParams) ([]domain.RegistrantResponse, domain.RegistrantListMeta, error)
	GetByID(admin domain.AdminProfile, id uuid.UUID) (domain.RegistrantResponse, error)
	UpdateStatus(admin domain.AdminProfile, registrantID uuid.UUID, input domain.UpdateRegistrantStatusInput) (domain.UpdateRegistrantStatusResult, error)
	OpenPortfolio(admin domain.AdminProfile, registrantID, documentID uuid.UUID) (entity.OprecStaffDocument, io.ReadCloser, error)
	Counts(admin domain.AdminProfile) (domain.RegistrantCountResponse, error)
	PrimarySubdivisionCounts(admin domain.AdminProfile) ([]domain.SubdivisionCountResponse, error)
	ActivityLogs(params domain.ActivityLogListParams) ([]domain.ActivityLogResponse, domain.RegistrantListMeta, error)
	Export(admin domain.AdminProfile, params domain.RegistrantListParams) (export.Table, error)
}

type oprecStaffService struct {
	repository repository.OprecStaffRepository
	config     config.OprecConfig
	storage    portfoliostorage.Store
	publicURL  string
	now        func() time.Time
}

func NewOprecStaff(
	repository repository.OprecStaffRepository,
	cfg config.OprecConfig,
	storage portfoliostorage.Store,
	publicURL string,
) OprecStaffService {
	return &oprecStaffService{repository: repository, config: cfg, storage: storage, publicURL: publicURL, now: time.Now}
}

func (s *oprecStaffService) Config(now time.Time) domain.OprecConfigResponse {
	return domain.OprecConfigResponse{
		SelectionFinalized: s.config.SelectionFinalized,
		RegistrationOpen:   s.config.RegistrationOpen(now),
		RegistrationStart:  utils.OptionalTime(s.config.RegistrationStartAt),
		RegistrationEnd:    utils.OptionalTime(s.config.RegistrationEndAt),
		AnnouncementOpen:   s.config.AnnouncementOpen(now),
		AnnouncementAt:     utils.OptionalTime(s.config.AnnouncementAt),
		ServerTime:         now,
		PortfolioMaxBytes:  s.config.PortfolioMaxBytes,
		AllowedBatches:     s.config.AllowedBatches,
		RequiredDocuments: []string{
			"KTM",
			"CV",
			"twibbon",
			"proof_of_follow_linkedin",
			"proof_of_follow_instagram",
			"proof_of_follow_tiktok",
		},
		PortfolioRequired: s.config.PortfolioRequired,
	}
}

func (s *oprecStaffService) List(admin domain.AdminProfile, params domain.RegistrantListParams) ([]domain.RegistrantResponse, domain.RegistrantListMeta, error) {
	divisionID, err := adminScope(admin)
	if err != nil {
		return nil, domain.RegistrantListMeta{}, err
	}
	registrants, total, err := s.repository.List(params, divisionID)
	if err != nil {
		return nil, domain.RegistrantListMeta{}, err
	}
	items := make([]domain.RegistrantResponse, 0, len(registrants))
	for _, registrant := range registrants {
		items = append(items, mapRegistrant(registrant, false))
	}
	return items, listMeta(params.Page, params.PageSize, total), nil
}

func (s *oprecStaffService) GetByID(admin domain.AdminProfile, id uuid.UUID) (domain.RegistrantResponse, error) {
	divisionID, err := adminScope(admin)
	if err != nil {
		return domain.RegistrantResponse{}, err
	}
	registrant, err := s.repository.GetByID(id, divisionID)
	if err != nil {
		return domain.RegistrantResponse{}, err
	}
	return mapRegistrant(registrant, true), nil
}

func (s *oprecStaffService) UpdateStatus(
	admin domain.AdminProfile,
	registrantID uuid.UUID,
	input domain.UpdateRegistrantStatusInput,
) (domain.UpdateRegistrantStatusResult, error) {
	scopeDivisionID, err := adminScope(admin)
	if err != nil {
		return domain.UpdateRegistrantStatusResult{}, err
	}
	if s.config.SelectionFinalized {
		return domain.UpdateRegistrantStatusResult{}, domain.ErrSelectionFinalized
	}
	status := entity.OprecStaffStatus(strings.ToLower(strings.TrimSpace(input.Status)))
	rejectionTarget := strings.ToLower(strings.TrimSpace(input.RejectionTarget))
	if rejectionTarget != "" {
		if rejectionTarget != "primary" && rejectionTarget != "secondary" && rejectionTarget != "both" {
			return domain.UpdateRegistrantStatusResult{}, myerror.NewFieldValidation("rejection_target", "Rejection", "must be primary, secondary, or both")
		}
		if input.Status != "" || input.AcceptedDivisionID != nil || input.CurrentSubdivisionID != nil {
			return domain.UpdateRegistrantStatusResult{}, myerror.NewFieldValidation("rejection_target", "Rejection", "cannot be combined with status or placement fields")
		}
	} else if !status.IsValid() {
		return domain.UpdateRegistrantStatusResult{}, domain.ErrInvalidRegistrantStatus
	}
	// Prevent the legacy final rejected status from bypassing choice ownership.
	if status == entity.OprecStaffStatusRejected && scopeDivisionID != nil {
		rejectionTarget = "both"
	}
	acceptedDivisionID, err := parsePlacementID(
		input.AcceptedDivisionID, status == entity.OprecStaffStatusAccepted,
		"accepted_division_id", "Accepted division", entity.OprecStaffStatusAccepted,
	)
	if err != nil {
		return domain.UpdateRegistrantStatusResult{}, err
	}
	currentSubdivisionID, err := parsePlacementID(
		input.CurrentSubdivisionID, status == entity.OprecStaffStatusInterview,
		"current_subdivision_id", "Current subdivision", entity.OprecStaffStatusInterview,
	)
	if err != nil {
		return domain.UpdateRegistrantStatusResult{}, err
	}

	audit, err := s.repository.UpdateStatus(repository.UpdateStatusParams{
		RegistrantID:         registrantID,
		ActorID:              admin.ID,
		ActorRole:            entity.UserRole(admin.Role),
		ActorDivisionID:      admin.DivisionID(),
		ScopeDivisionID:      scopeDivisionID,
		Status:               status,
		AcceptedDivisionID:   acceptedDivisionID,
		CurrentSubdivisionID: currentSubdivisionID,
		RejectionTarget:      rejectionTarget,
	})
	if err != nil {
		return domain.UpdateRegistrantStatusResult{}, err
	}
	return domain.UpdateRegistrantStatusResult{
		RegistrationID:               registrantID,
		PreviousStatus:               string(audit.PreviousStatus),
		Status:                       string(audit.NewStatus),
		PrimaryRejected:              audit.NewPrimaryRejected,
		SecondaryRejected:            audit.NewSecondaryRejected,
		PreviousDivisionID:           audit.PreviousAcceptedSubdivisionID,
		AcceptedDivisionID:           audit.NewAcceptedSubdivisionID,
		PreviousCurrentSubdivisionID: audit.PreviousCurrentSubdivisionID,
		CurrentSubdivisionID:         audit.NewCurrentSubdivisionID,
	}, nil
}

// parsePlacementID mem-parse ID subdivisi penempatan yang hanya boleh diisi
// pada status tertentu (accepted atau interview). Kalau allowed true, field wajib
// diisi; kalau false, field harus kosong.
func parsePlacementID(
	raw *string,
	allowed bool,
	field, label string,
	requiredStatus entity.OprecStaffStatus,
) (*uuid.UUID, error) {
	value := ""
	if raw != nil {
		value = strings.TrimSpace(*raw)
	}
	if !allowed {
		if value != "" {
			return nil, myerror.NewFieldValidation(field, label, "is only allowed when status is "+string(requiredStatus))
		}
		return nil, nil
	}
	if value == "" {
		return nil, myerror.NewFieldValidation(field, label, "is required when status is "+string(requiredStatus))
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil, myerror.NewFieldValidation(field, label, "must be a valid UUID")
	}
	return &parsed, nil
}

func (s *oprecStaffService) OpenPortfolio(
	admin domain.AdminProfile, registrantID, documentID uuid.UUID,
) (entity.OprecStaffDocument, io.ReadCloser, error) {
	divisionID, err := adminScope(admin)
	if err != nil {
		return entity.OprecStaffDocument{}, nil, err
	}
	document, err := s.repository.GetDocument(registrantID, documentID, divisionID)
	if err != nil {
		return entity.OprecStaffDocument{}, nil, err
	}
	if document.Tipe != domain.DocumentTypePortfolio {
		return entity.OprecStaffDocument{}, nil, domain.ErrDocumentNotFound
	}
	file, err := s.storage.Open(document.URL)
	if err != nil {
		return entity.OprecStaffDocument{}, nil, domain.ErrDocumentNotFound
	}
	return document, file, nil
}

func (s *oprecStaffService) Counts(admin domain.AdminProfile) (domain.RegistrantCountResponse, error) {
	divisionID, err := adminScope(admin)
	if err != nil {
		return domain.RegistrantCountResponse{}, err
	}
	return s.repository.Counts(divisionID)
}

func (s *oprecStaffService) PrimarySubdivisionCounts(admin domain.AdminProfile) ([]domain.SubdivisionCountResponse, error) {
	divisionID, err := adminScope(admin)
	if err != nil {
		return nil, err
	}
	return s.repository.PrimarySubdivisionCounts(divisionID)
}

func (s *oprecStaffService) ActivityLogs(
	params domain.ActivityLogListParams,
) ([]domain.ActivityLogResponse, domain.RegistrantListMeta, error) {
	logs, total, err := s.repository.ListActivityLogs(params)
	if err != nil {
		return nil, domain.RegistrantListMeta{}, err
	}
	items := make([]domain.ActivityLogResponse, 0, len(logs))
	for _, log := range logs {
		items = append(items, mapActivityLog(log))
	}
	return items, listMeta(params.Page, params.PageSize, total), nil
}

func listMeta(page, pageSize int, total int64) domain.RegistrantListMeta {
	return domain.RegistrantListMeta{
		Page: page, PageSize: pageSize, TotalItems: total, TotalPages: utils.TotalPages(total, pageSize),
	}
}

func adminScope(admin domain.AdminProfile) (*uuid.UUID, error) {
	switch entity.UserRole(admin.Role) {
	case entity.RoleSuperAdmin:
		return nil, nil
	case entity.RoleDivisionAdmin:
		if admin.Division == nil {
			return nil, domain.ErrForbidden
		}
		return admin.DivisionID(), nil
	default:
		return nil, domain.ErrForbidden
	}
}
