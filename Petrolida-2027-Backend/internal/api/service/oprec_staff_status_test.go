package service

import (
	"errors"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func TestFinalizedStaffSelectionIsReadOnlyForEveryAdmin(t *testing.T) {
	now := time.Now()
	svc := &oprecStaffService{config: config.OprecConfig{
		SelectionFinalized:  true,
		RegistrationStartAt: now.Add(-time.Hour), RegistrationEndAt: now.Add(time.Hour),
	}}
	for _, role := range []entity.UserRole{entity.RoleSuperAdmin, entity.RoleDivisionAdmin} {
		admin := domain.AdminProfile{Role: string(role), Division: &domain.MasterReference{ID: uuid.New()}}
		_, err := svc.UpdateStatus(admin, uuid.New(), domain.UpdateRegistrantStatusInput{Status: "pending"})
		if !errors.Is(err, domain.ErrSelectionFinalized) {
			t.Fatalf("%s update = %v", role, err)
		}
	}
	if cfg := svc.Config(now); !cfg.SelectionFinalized || cfg.RegistrationOpen {
		t.Fatalf("finalized configuration = %+v", cfg)
	}
	// No repository is needed: a finalized selection closes registration before
	// validation, file uploads, or writes can take place.
	svc.now = func() time.Time { return now }
	if _, err := svc.Register(domain.RegistrationInput{}, nil); !errors.Is(err, domain.ErrRegistrationClosed) {
		t.Fatalf("registration after finalization = %v", err)
	}
}

type decisionRepository struct {
	repository.OprecStaffRepository
	target string
}

func (r *decisionRepository) UpdateStatus(params repository.UpdateStatusParams) (entity.OprecStaffStatusAudit, error) {
	r.target = params.RejectionTarget
	return entity.OprecStaffStatusAudit{PreviousStatus: entity.OprecStaffStatusScreeningPrimary, NewStatus: entity.OprecStaffStatusScreeningSecondary, NewPrimaryRejected: true}, nil
}

func TestUpdateStatusRejectsAmbiguousOrInvalidRejections(t *testing.T) {
	// The embedded repository will panic if a rejected payload reaches storage.
	svc := &oprecStaffService{repository: &decisionRepository{}}
	admin := domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}
	placement := uuid.NewString()
	for _, input := range []domain.UpdateRegistrantStatusInput{
		{RejectionTarget: "other"},
		{RejectionTarget: "primary", Status: "accepted"},
		{RejectionTarget: "primary", AcceptedDivisionID: &placement},
		{RejectionTarget: "both", CurrentSubdivisionID: &placement},
	} {
		if _, err := svc.UpdateStatus(admin, uuid.New(), input); err == nil {
			t.Fatalf("invalid payload accepted: %+v", input)
		}
	}
}

func TestUpdateStatusUsesRepositoryDecisionForHandoff(t *testing.T) {
	repo := &decisionRepository{}
	result, err := (&oprecStaffService{repository: repo}).UpdateStatus(domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}, uuid.New(), domain.UpdateRegistrantStatusInput{RejectionTarget: "primary"})
	if err != nil || repo.target != "primary" || result.Status != "screening_secondary" || !result.PrimaryRejected || result.SecondaryRejected {
		t.Fatalf("result = %+v %v, target=%s", result, err, repo.target)
	}
	division := &domain.MasterReference{ID: uuid.New()}
	_, err = (&oprecStaffService{repository: repo}).UpdateStatus(domain.AdminProfile{Role: string(entity.RoleDivisionAdmin), Division: division}, uuid.New(), domain.UpdateRegistrantStatusInput{Status: "rejected"})
	if err != nil || repo.target != "both" {
		t.Fatalf("legacy rejected bypassed ownership check: target=%s error=%v", repo.target, err)
	}
}
