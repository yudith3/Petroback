package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

type confirmationRepo struct {
	repository.OprecStaffRepository
	saved   *entity.OprecStaff
	err     error
	creates int
	race    bool
}

func (r *confirmationRepo) GetBySubmissionKey(_ context.Context, key uuid.UUID) (entity.OprecStaff, error) {
	if r.err != nil {
		return entity.OprecStaff{}, r.err
	}
	if r.saved == nil || r.saved.SubmissionKey == nil || *r.saved.SubmissionKey != key {
		return entity.OprecStaff{}, domain.ErrRegistrantNotFound
	}
	return *r.saved, nil
}
func (r *confirmationRepo) ValidateReferences(_, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]entity.Subdivisi, error) {
	result := map[uuid.UUID]entity.Subdivisi{}
	for _, id := range ids {
		result[id] = entity.Subdivisi{ID: id, Nama: "Fixture"}
	}
	return result, nil
}
func (r *confirmationRepo) Create(value *entity.OprecStaff, _ []entity.OprecStaffChoice, _ *entity.OprecStaffSkill, _ *entity.OprecStaffAdditionalInformation, _ []entity.OprecStaffDocument) error {
	r.creates++
	value.ID = uuid.New()
	copy := *value
	r.saved = &copy
	if r.race {
		return domain.ErrRegistrationExists
	}
	return nil
}
func confirmationFixture() domain.RegistrationInput {
	no := false
	return domain.RegistrationInput{
		SubmissionKey: uuid.NewString(), Name: "Fixture Applicant", Email: "fixture@example.test", NRP: "0000000000", PhoneNumber: "+6281234567890", Batch: 2026,
		FacultyID: uuid.NewString(), DepartmentID: uuid.NewString(), AboutPetrolida: "Fixture answer", WillingTransferDivision: &no,
		RequiredDocumentURL: "https://drive.google.com/drive/folders/fixture",
		Choices:             []domain.RegistrationChoiceInput{{Priority: 1, SubdivisionID: uuid.NewString(), AboutDivision: "Fixture", ReasonToJoin: "Fixture", InnovationIdea: "Fixture"}, {Priority: 2, SubdivisionID: uuid.NewString(), AboutDivision: "Fixture", ReasonToJoin: "Fixture", InnovationIdea: "Fixture"}},
		Skills:              domain.RegistrationSkillInput{EnglishProficiency: "good", CommunicationSkill: "good", CreativityInEvent: "good", FastResponse: "good", GraphicDesign: "good", Photography: "good", VideoEditing: "good", WebDesign: "good", OperatingZoom: "good", SocialMediaTrends: "good", DesignApplications: "Fixture"},
		AdditionalInfo:      domain.RegistrationAdditionalInformationInput{HasCarInSurabaya: &no, HasSponsorRelation: &no, HasVendorInSurabaya: &no, HasCamera: &no, HasLaptop: &no, HasTablet: &no, ConnectedCompany: "None"},
	}
}
func confirmationService(repo *confirmationRepo) *oprecStaffService {
	return &oprecStaffService{repository: repo, config: config.OprecConfig{RegistrationStartAt: time.Now().Add(-time.Hour), RegistrationEndAt: time.Now().Add(time.Hour), AllowedBatches: []int16{2026}, PortfolioMaxBytes: 10 * 1024 * 1024}, now: time.Now}
}
func TestRegistrationRetryReturnsSameConfirmation(t *testing.T) {
	for _, race := range []bool{false, true} {
		repo := &confirmationRepo{race: race}
		svc := confirmationService(repo)
		input := confirmationFixture()
		first, err := svc.Register(input, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Recovery remains available after the registration deadline.
		svc.config.RegistrationEndAt = time.Now().Add(-time.Minute)
		second, err := svc.Register(input, nil)
		if err != nil || first.RegistrationID != second.RegistrationID || repo.creates != 1 {
			t.Fatalf("retry = %#v, %v; creates=%d", second, err, repo.creates)
		}
		input.Name = "Changed answer"
		if _, err := svc.Register(input, nil); !errors.Is(err, domain.ErrSubmissionKeyConflict) {
			t.Fatalf("changed retry: %v", err)
		}
	}
}
func TestRegistrationConfirmationDoesNotInferFailureOrExposeSelection(t *testing.T) {
	repo := &confirmationRepo{}
	svc := confirmationService(repo)
	key := uuid.New()
	result, err := svc.ConfirmRegistration(context.Background(), key.String())
	if err != nil || result.State != "unconfirmed" || result.Registration != nil {
		t.Fatalf("unknown submission = %#v, %v", result, err)
	}
	repo.saved = &entity.OprecStaff{ID: uuid.New(), SubmissionKey: &key, Status: entity.OprecStaffStatusAccepted, Nama: "Private name"}
	result, err = svc.ConfirmRegistration(context.Background(), key.String())
	if err != nil || result.State != "confirmed" || result.Registration.Status != "pending" {
		t.Fatalf("confirmed = %#v, %v", result, err)
	}
	repo.err = errors.New("database unavailable")
	if _, err := svc.ConfirmRegistration(context.Background(), key.String()); !errors.Is(err, repo.err) {
		t.Fatalf("database error hidden: %v", err)
	}
	if _, err := svc.ConfirmRegistration(context.Background(), "00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatal("accepted predictable key")
	}
}
func TestSubmissionFingerprintIncludesPortfolioContents(t *testing.T) {
	svc := confirmationService(&confirmationRepo{})
	input := confirmationFixture()
	first := multipartFileHeader(t, "portfolio.pdf", []byte("%PDF-first"))
	second := multipartFileHeader(t, "portfolio.pdf", []byte("%PDF-other"))
	a, err := svc.registrationFingerprint(input, first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.registrationFingerprint(input, second)
	if err != nil || a == b {
		t.Fatalf("different files shared a fingerprint: %v", err)
	}
}
