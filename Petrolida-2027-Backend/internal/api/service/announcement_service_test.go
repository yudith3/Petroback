package service

import (
	"context"
	"errors"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/google/uuid"
	"testing"
	"time"
)

type announcementRepoStub struct {
	applicant      entity.OprecStaff
	err            error
	calls          int
	interviewCalls int
	snapshot       entity.InterviewResult
	snapshotErr    error
}

func (r *announcementRepoStub) GetInterviewResult(_ context.Context, nrp string) (entity.InterviewResult, error) {
	r.interviewCalls++
	if r.snapshotErr != nil {
		return entity.InterviewResult{}, r.snapshotErr
	}
	if r.snapshot.NRP != nrp {
		return entity.InterviewResult{}, domain.ErrRegistrantNotFound
	}
	return r.snapshot, nil
}

func (r *announcementRepoStub) GetByNRP(_ context.Context, nrp string) (entity.OprecStaff, error) {
	r.calls++
	if nrp != "5070261234" {
		return entity.OprecStaff{}, domain.ErrRegistrantNotFound
	}
	return r.applicant, r.err
}
func TestAnnouncementLookup(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	sub := &entity.Subdivisi{ID: uuid.New(), Nama: "Web Development"}
	cases := []struct {
		name, stage string
		status      entity.OprecStaffStatus
		subdivision *entity.Subdivisi
		want        string
		wantErr     error
	}{
		{"staff accepted", "staff", entity.OprecStaffStatusAccepted, sub, "accepted", nil},
		{"staff rejected", "staff", entity.OprecStaffStatusRejected, nil, "rejected", nil},
		{"staff pending interview", "staff", entity.OprecStaffStatusInterview, sub, "", domain.ErrAnnouncementPending},
		{"screening pending", "staff", entity.OprecStaffStatusScreeningPrimary, sub, "", domain.ErrAnnouncementPending},
		{"missing assignment", "staff", entity.OprecStaffStatusAccepted, nil, "", domain.ErrAnnouncementPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &announcementRepoStub{applicant: entity.OprecStaff{Nama: "Applicant", NRP: "5070261234", Status: tc.status, AcceptedDivisi: tc.subdivision, CurrentSubdivision: tc.subdivision}}
			svc := NewAnnouncement(repo, config.OprecConfig{AnnouncementAt: now, InterviewAnnouncementAt: now, StaffGroupURL: "https://example.com/group", InterviewScheduleURL: "https://example.com/schedule"})
			result, err := svc.Lookup(context.Background(), tc.stage, domain.AnnouncementInput{NRP: " 5070261234 "}, now)
			if !sameAnnouncementError(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if result.Status != tc.want {
				t.Fatalf("status = %q, want %q", result.Status, tc.want)
			}
			if tc.want == "accepted" && (result.Subdivision == nil || result.ActionURL == "") {
				t.Fatal("accepted result missing subdivision or action")
			}
			if tc.want == "rejected" && (result.Subdivision != nil || result.ActionURL != "") {
				t.Fatal("rejected result exposes action or subdivision")
			}
		})
	}
}
func TestAnnouncementGateBeforeDatabaseLookup(t *testing.T) {
	now := time.Now()
	repo := &announcementRepoStub{}
	for _, at := range []time.Time{{}, now.Add(time.Hour)} {
		svc := NewAnnouncement(repo, config.OprecConfig{AnnouncementAt: at, InterviewAnnouncementAt: at})
		for _, stage := range []string{"staff", "interview"} {
			_, err := svc.Lookup(context.Background(), stage, domain.AnnouncementInput{NRP: "5070261234"}, now)
			if !sameAnnouncementError(err, domain.ErrAnnouncementClosed) {
				t.Fatalf("error = %v", err)
			}
		}
	}
	if repo.calls != 0 || repo.interviewCalls != 0 {
		t.Fatal("closed announcement queried applicant data")
	}
	svc := NewAnnouncement(repo, config.OprecConfig{AnnouncementAt: now})
	for _, nrp := range []string{"", "123", "507026123x", "50702612345"} {
		if _, err := svc.Lookup(context.Background(), "staff", domain.AnnouncementInput{NRP: nrp}, now); err == nil {
			t.Fatalf("accepted invalid NRP %q", nrp)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid NRP queried applicant data")
	}
	_, err := svc.Lookup(context.Background(), "staff", domain.AnnouncementInput{NRP: "5070261235"}, now)
	if !sameAnnouncementError(err, domain.ErrRegistrantNotFound) {
		t.Fatalf("missing applicant error = %v", err)
	}
	repo.err = errors.New("database unavailable")
	_, err = svc.Lookup(context.Background(), "staff", domain.AnnouncementInput{NRP: "5070261234"}, now)
	if !errors.Is(err, repo.err) {
		t.Fatalf("database error = %v", err)
	}
}

func TestInterviewUsesPublishedRoster(t *testing.T) {
	now := time.Now()
	sub := &entity.Subdivisi{ID: uuid.New(), Nama: "Logistic"}
	for _, liveStatus := range []entity.OprecStaffStatus{entity.OprecStaffStatusPending, entity.OprecStaffStatusInterview, entity.OprecStaffStatusAccepted, entity.OprecStaffStatusRejected} {
		t.Run(string(liveStatus), func(t *testing.T) {
			repo := &announcementRepoStub{
				applicant: entity.OprecStaff{Nama: "Live name", NRP: "5070261234", Status: liveStatus},
				snapshot:  entity.InterviewResult{NRP: "5070261234", Name: "Published name", Status: "accepted", Subdivision: sub},
			}
			svc := NewAnnouncement(repo, config.OprecConfig{InterviewAnnouncementAt: now, InterviewScheduleURL: "https://example.com/schedule"})
			result, err := svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: " 5070261234 "}, now)
			if err != nil || result.Status != "accepted" || result.Name != "Published name" || result.Subdivision != nil || result.ActionURL != "https://example.com/schedule" {
				t.Fatalf("result = %+v, error = %v", result, err)
			}
			if repo.calls != 0 {
				t.Fatal("published acceptance depends on live registrant")
			}
		})
	}
}

func TestInterviewOutsideRosterIsRejectedOrNotFound(t *testing.T) {
	now := time.Now()
	for _, status := range []entity.OprecStaffStatus{entity.OprecStaffStatusPending, entity.OprecStaffStatusInterview, entity.OprecStaffStatusAccepted, entity.OprecStaffStatusRejected} {
		repo := &announcementRepoStub{applicant: entity.OprecStaff{Nama: "Known applicant", NRP: "5070261234", Status: status}}
		svc := NewAnnouncement(repo, config.OprecConfig{InterviewAnnouncementAt: now})
		result, err := svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: "5070261234"}, now)
		if err != nil || result.Status != "rejected" || result.Name != "Known applicant" || result.Subdivision != nil || result.ActionURL != "" {
			t.Fatalf("known outsider = %+v, error = %v", result, err)
		}
		_, err = svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: "5070261235"}, now)
		if !sameAnnouncementError(err, domain.ErrRegistrantNotFound) {
			t.Fatalf("unknown outsider = %v", err)
		}
	}
}

func TestInterviewOnHoldAndDatabaseFailure(t *testing.T) {
	now := time.Now()
	repo := &announcementRepoStub{snapshot: entity.InterviewResult{NRP: "5070261234", Status: "on_hold"}}
	svc := NewAnnouncement(repo, config.OprecConfig{InterviewAnnouncementAt: now})
	result, err := svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: "5070261234"}, now)
	if !sameAnnouncementError(err, domain.ErrAnnouncementOnHold) || result.Status != "" || repo.calls != 0 {
		t.Fatalf("held result = %+v, error = %v", result, err)
	}
	repo.snapshotErr = errors.New("snapshot database unavailable")
	_, err = svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: "5070261234"}, now)
	if !errors.Is(err, repo.snapshotErr) || repo.calls != 0 {
		t.Fatalf("snapshot error = %v", err)
	}
	repo.snapshotErr = nil
	repo.snapshot = entity.InterviewResult{}
	repo.err = errors.New("registrant database unavailable")
	_, err = svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: "5070261234"}, now)
	if !errors.Is(err, repo.err) {
		t.Fatalf("registrant error = %v", err)
	}
}

func TestInterviewRetainsExplicitFifteenDigitID(t *testing.T) {
	now := time.Now()
	repo := &announcementRepoStub{snapshot: entity.InterviewResult{NRP: "351518190408001", Status: "accepted"}}
	svc := NewAnnouncement(repo, config.OprecConfig{InterviewAnnouncementAt: now})
	result, err := svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: " 351518190408001 "}, now)
	if err != nil || result.NRP != "351518190408001" || result.Status != "accepted" {
		t.Fatalf("retained ID = %+v, error = %v", result, err)
	}
	for _, nrp := range []string{"351518190408002", "501251134", "505625049", "35151819040800"} {
		_, err = svc.Lookup(context.Background(), "interview", domain.AnnouncementInput{NRP: nrp}, now)
		var validation myerror.Error
		if !errors.As(err, &validation) || validation.Code != "VALIDATION_ERROR" {
			t.Fatalf("invalid ID %s = %v", nrp, err)
		}
	}
}

func sameAnnouncementError(got, want error) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	var actual, expected myerror.Error
	if errors.As(got, &actual) && errors.As(want, &expected) {
		return actual.Code == expected.Code
	}
	return errors.Is(got, want)
}
