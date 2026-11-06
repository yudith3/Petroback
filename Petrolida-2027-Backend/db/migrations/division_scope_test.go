package migrations

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm/logger"
)

func TestInterviewDivisionScopePreservesLegacyAccess(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set; PostgreSQL division scope test runs in CI")
	}
	admin := openMigrationTestDB(t, "")
	schema := fmt.Sprintf("division_scope_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", schema)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema)).Error
		closeMigrationTestDB(t, admin)
	})
	database := openMigrationTestDB(t, schema)
	t.Cleanup(func() { closeMigrationTestDB(t, database) })
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	// Out-of-scope detail lookups deliberately return record-not-found errors.
	database.Logger = logger.Default.LogMode(logger.Silent)
	faculty := entity.Fakultas{Nama: "Fixture faculty"}
	if err := database.Create(&faculty).Error; err != nil {
		t.Fatal(err)
	}
	department := entity.Departemen{Nama: "Fixture department", FakultasID: faculty.ID}
	if err := database.Create(&department).Error; err != nil {
		t.Fatal(err)
	}
	divisions := []entity.Divisi{{Nama: "Finance"}, {Nama: "Other"}, {Nama: "Unrelated"}}
	if err := database.Create(&divisions).Error; err != nil {
		t.Fatal(err)
	}
	financeID, otherID := divisions[0].ID, divisions[1].ID
	subdivisions := []entity.Subdivisi{
		{Nama: "Sponsorship", DivisiID: financeID},
		{Nama: "Fundraise", DivisiID: financeID},
		{Nama: "Other subdivision", DivisiID: otherID},
		{Nama: "Deleted finance subdivision", DivisiID: financeID},
	}
	if err := database.Create(&subdivisions).Error; err != nil {
		t.Fatal(err)
	}
	sponsorshipID, fundraiseID, otherSubdivisionID, deletedSubdivisionID :=
		subdivisions[0].ID, subdivisions[1].ID, subdivisions[2].ID, subdivisions[3].ID
	if err := database.Delete(&subdivisions[3]).Error; err != nil {
		t.Fatal(err)
	}
	type scopeCase struct {
		name          string
		status        entity.OprecStaffStatus
		choices       []uuid.UUID
		current       *uuid.UUID
		accepted      *uuid.UUID
		visible       []uuid.UUID
		deletedChoice bool
		deleted       bool
		id            uuid.UUID
	}
	cases := []scopeCase{
		{name: "legacy_finance", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{sponsorshipID, fundraiseID}, visible: []uuid.UUID{financeID}},
		{name: "legacy_finance_secondary", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{otherSubdivisionID, fundraiseID}, visible: []uuid.UUID{financeID, otherID}},
		{name: "assigned_finance", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{otherSubdivisionID}, current: &sponsorshipID, visible: []uuid.UUID{financeID}},
		{name: "assigned_other", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{sponsorshipID}, current: &otherSubdivisionID, visible: []uuid.UUID{otherID}},
		{name: "pending_finance", status: entity.OprecStaffStatusPending, choices: []uuid.UUID{sponsorshipID}, visible: []uuid.UUID{financeID}},
		{name: "accepted_other", status: entity.OprecStaffStatusAccepted, choices: []uuid.UUID{sponsorshipID}, accepted: &otherSubdivisionID, visible: []uuid.UUID{otherID}},
		{name: "accepted_unassigned", status: entity.OprecStaffStatusAccepted, choices: []uuid.UUID{sponsorshipID}},
		{name: "rejected_finance", status: entity.OprecStaffStatusRejected, choices: []uuid.UUID{sponsorshipID}, visible: []uuid.UUID{financeID}},
		{name: "legacy_other", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{otherSubdivisionID}, visible: []uuid.UUID{otherID}},
		{name: "deleted_finance_choice", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{sponsorshipID, otherSubdivisionID}, deletedChoice: true, visible: []uuid.UUID{otherID}},
		{name: "deleted_subdivision", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{deletedSubdivisionID}},
		{name: "deleted_registrant", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{sponsorshipID}, deleted: true},
		{name: "assigned_deleted_subdivision", status: entity.OprecStaffStatusInterview, choices: []uuid.UUID{sponsorshipID}, current: &deletedSubdivisionID},
	}
	for index := range cases {
		testCase := &cases[index]
		registrant := entity.OprecStaff{
			Nama: testCase.name, Email: fmt.Sprintf("scope%d@example.test", index),
			NRP: fmt.Sprintf("000100%04d", index), NomorTelepon: "+6281234567890",
			FakultasID: faculty.ID, DepartemenID: department.ID, Angkatan: 2026,
			TentangPetrolida: "Fixture", Status: testCase.status,
			CurrentSubdivisionID: testCase.current, AcceptedDivisiID: testCase.accepted,
		}
		if err := database.Create(&registrant).Error; err != nil {
			t.Fatal(err)
		}
		testCase.id = registrant.ID
		for priority, subdivisionID := range testCase.choices {
			choice := entity.OprecStaffChoice{OprecStaffID: registrant.ID, Pilihan: int16(priority + 1), SubdivisiID: subdivisionID}
			if err := database.Create(&choice).Error; err != nil {
				t.Fatal(err)
			}
			if priority == 0 && testCase.deletedChoice {
				if err := database.Delete(&choice).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
		if testCase.deleted {
			if err := database.Delete(&registrant).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	repo := repository.NewOprecStaff(database)
	for _, scope := range []struct {
		name string
		id   *uuid.UUID
	}{{"finance", &financeID}, {"other", &otherID}, {"unrelated", &divisions[2].ID}, {"superadmin", nil}} {
		t.Run(scope.name, func(t *testing.T) {
			expected := make(map[uuid.UUID]bool)
			var expectedInterviews int64
			for _, testCase := range cases {
				visible := scope.id == nil
				for _, divisionID := range testCase.visible {
					visible = visible || (scope.id != nil && *scope.id == divisionID)
				}
				visible = visible && !testCase.deleted
				if visible {
					expected[testCase.id] = true
					if testCase.status == entity.OprecStaffStatusInterview {
						expectedInterviews++
					}
				}
				_, err := repo.GetByID(testCase.id, scope.id)
				if visible && err != nil {
					t.Errorf("%s detail is inaccessible: %v", testCase.name, err)
				}
				if !visible && !errors.Is(err, domain.ErrRegistrantNotFound) {
					t.Errorf("%s detail bypassed scope: %v", testCase.name, err)
				}
			}
			list, total, err := repo.List(domain.RegistrantListParams{Page: 1, PageSize: 50}, scope.id)
			if err != nil {
				t.Fatal(err)
			}
			if total != int64(len(expected)) || len(list) != len(expected) {
				t.Fatalf("list contains %d/%d records, want %d", len(list), total, len(expected))
			}
			for _, registrant := range list {
				if !expected[registrant.ID] {
					t.Errorf("list exposed out-of-scope record %s", registrant.ID)
				}
			}
			counts, err := repo.Counts(scope.id)
			if err != nil {
				t.Fatal(err)
			}
			if counts.Register != int64(len(expected)) || counts.Interviewed != expectedInterviews {
				t.Errorf("counts disagree with visibility: %+v", counts)
			}
		})
	}
	t.Run("finance_can_assign_legacy_interview", func(t *testing.T) {
		actor := entity.User{
			Name: "Fixture finance admin", Email: "finance@example.test", Password: "fixture",
			Instansi: "Fixture", NoTelp: "+6281234567890", Role: entity.RoleDivisionAdmin, DivisiID: &financeID,
		}
		if err := database.Create(&actor).Error; err != nil {
			t.Fatal(err)
		}
		// The legacy applicant is shared through both original divisions until assigned.
		legacy := cases[1]
		_, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: legacy.id, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &financeID, ScopeDivisionID: &financeID, Status: entity.OprecStaffStatusInterview, CurrentSubdivisionID: &otherSubdivisionID})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("finance assigned another division's subdivision: %v", err)
		}
		audit, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: legacy.id, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &financeID, ScopeDivisionID: &financeID, Status: entity.OprecStaffStatusInterview, CurrentSubdivisionID: &fundraiseID})
		if err != nil {
			t.Fatal(err)
		}
		if audit.NewCurrentSubdivisionID == nil || *audit.NewCurrentSubdivisionID != fundraiseID {
			t.Fatalf("assignment was not audited: %+v", audit)
		}
		saved, err := repo.GetByID(legacy.id, &financeID)
		if err != nil || saved.CurrentSubdivisionID == nil || *saved.CurrentSubdivisionID != fundraiseID {
			t.Fatalf("finance lost assigned interview: %v", err)
		}
		if _, err := repo.GetByID(legacy.id, &otherID); !errors.Is(err, domain.ErrRegistrantNotFound) {
			t.Fatalf("original other choice retained access after assignment: %v", err)
		}
	})
}
