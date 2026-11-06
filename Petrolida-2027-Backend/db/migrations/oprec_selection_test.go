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
)

func TestChoiceRejectionsAndFiltersInPostgres(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set")
	}
	admin := openMigrationTestDB(t, "")
	schema := fmt.Sprintf("choice_selection_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", schema)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema)).Error
		closeMigrationTestDB(t, admin)
	})
	database := openMigrationTestDB(t, schema)
	t.Cleanup(func() { closeMigrationTestDB(t, database) })
	for i := 0; i < 2; i++ {
		if err := Migrate(database); err != nil {
			t.Fatal(err)
		}
	}
	faculty := entity.Fakultas{Nama: "Fixture"}
	if err := database.Create(&faculty).Error; err != nil {
		t.Fatal(err)
	}
	department := entity.Departemen{Nama: "Fixture", FakultasID: faculty.ID}
	if err := database.Create(&department).Error; err != nil {
		t.Fatal(err)
	}
	divisions := []entity.Divisi{{Nama: "Primary", IsActive: true}, {Nama: "Secondary", IsActive: true}}
	if err := database.Create(&divisions).Error; err != nil {
		t.Fatal(err)
	}
	primaryDivision, secondaryDivision := divisions[0].ID, divisions[1].ID
	subdivisions := []entity.Subdivisi{{Nama: "P1", DivisiID: primaryDivision, IsActive: true}, {Nama: "P2", DivisiID: primaryDivision, IsActive: true}, {Nama: "S", DivisiID: secondaryDivision, IsActive: true}}
	if err := database.Create(&subdivisions).Error; err != nil {
		t.Fatal(err)
	}
	actor := entity.User{Name: "Admin", Email: "admin@petrolida.com", Password: "fixture", Instansi: "Fixture", NoTelp: "081234567890", Role: entity.RoleSuperAdmin}
	if err := database.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewOprecStaff(database)
	registrants := make([]entity.OprecStaff, 3)
	choiceIDs := [][]uuid.UUID{{subdivisions[0].ID, subdivisions[2].ID}, {subdivisions[2].ID, subdivisions[1].ID}, {subdivisions[0].ID, subdivisions[1].ID}}
	for i := range registrants {
		registrants[i] = entity.OprecStaff{Nama: fmt.Sprintf("Applicant %d", i), Email: fmt.Sprintf("applicant%d@example.test", i), NRP: fmt.Sprintf("000%d", i), NomorTelepon: "081234567890", FakultasID: faculty.ID, DepartemenID: department.ID, Angkatan: 2026, Status: entity.OprecStaffStatusPending}
		if err := database.Create(&registrants[i]).Error; err != nil {
			t.Fatal(err)
		}
		for j, id := range choiceIDs[i] {
			if err := database.Create(&entity.OprecStaffChoice{OprecStaffID: registrants[i].ID, Pilihan: int16(j + 1), SubdivisiID: id}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := database.Create(&entity.OprecStaffSkill{
		OprecStaffID: registrants[0].ID, EnglishProficiency: entity.OprecStaffSkillLevelGood,
		CommunicationSkill: entity.OprecStaffSkillLevelGood, CreativityInEvent: entity.OprecStaffSkillLevelGood,
		FastResponse: entity.OprecStaffSkillLevelGood, GraphicDesign: entity.OprecStaffSkillLevelGood,
		Photography: entity.OprecStaffSkillLevelGood, VideoEditing: entity.OprecStaffSkillLevelGood,
		WebDesign: entity.OprecStaffSkillLevelGood, OperatingZoom: entity.OprecStaffSkillLevelGood,
		SocialMediaTrends: entity.OprecStaffSkillLevelGood, DesignApplications: "Figma",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&entity.OprecStaffAdditionalInformation{
		OprecStaffID: registrants[0].ID, HasLaptop: true, HasSponsorRelation: true, ConnectedCompany: "Fixture company",
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Run("export_loads_all_registration_responses", func(t *testing.T) {
		exported, err := repo.ListForExport(domain.RegistrantListParams{Search: registrants[0].NRP, Page: 1, PageSize: 1}, nil)
		if err != nil || len(exported) != 1 {
			t.Fatalf("export count = %d, err = %v", len(exported), err)
		}
		staff := exported[0]
		if staff.Skills == nil || staff.Skills.EnglishProficiency != entity.OprecStaffSkillLevelGood || staff.Skills.DesignApplications != "Figma" {
			t.Fatalf("export omitted skills: %+v", staff.Skills)
		}
		if staff.AdditionalInfo == nil || !staff.AdditionalInfo.HasLaptop || !staff.AdditionalInfo.HasSponsorRelation || staff.AdditionalInfo.ConnectedCompany != "Fixture company" {
			t.Fatalf("export omitted additional information: %+v", staff.AdditionalInfo)
		}
		if len(staff.Choices) != 2 || staff.Choices[0].Subdivisi.Divisi == nil || staff.Choices[1].Subdivisi.Divisi == nil {
			t.Fatalf("export omitted choice relations: %+v", staff.Choices)
		}
	})
	for _, test := range []struct {
		priority int16
		want     int64
	}{{0, 3}, {1, 2}, {2, 2}} {
		params := domain.RegistrantListParams{Page: 1, PageSize: 1, DivisionID: &primaryDivision, ChoicePriority: test.priority}
		list, total, err := repo.List(params, nil)
		if err != nil || total != test.want || len(list) != 1 {
			t.Fatalf("priority %d: list=%d total=%d err=%v", test.priority, len(list), total, err)
		}
		exported, err := repo.ListForExport(params, nil)
		if err != nil || int64(len(exported)) != test.want {
			t.Fatalf("export priority %d: count=%d err=%v", test.priority, len(exported), err)
		}
	}
	id := registrants[0].ID
	if _, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: id, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &primaryDivision, ScopeDivisionID: &primaryDivision, RejectionTarget: "both"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("cross-division reject both: %v", err)
	}
	audit, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: id, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &primaryDivision, ScopeDivisionID: &primaryDivision, RejectionTarget: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if audit.PreviousStatus != entity.OprecStaffStatusPending || audit.NewStatus != entity.OprecStaffStatusScreeningSecondary || !audit.NewPrimaryRejected || audit.NewSecondaryRejected || audit.RejectionTarget != "primary" {
		t.Fatalf("wrong handoff audit: %+v", audit)
	}
	if _, err := repo.GetByID(id, &primaryDivision); !errors.Is(err, domain.ErrRegistrantNotFound) {
		t.Fatalf("primary retained handoff: %v", err)
	}
	saved, err := repo.GetByID(id, &secondaryDivision)
	if err != nil || saved.CurrentSubdivisionID == nil || *saved.CurrentSubdivisionID != subdivisions[2].ID {
		t.Fatalf("secondary did not receive applicant: %+v %v", saved, err)
	}
	// Accepting the remaining choice keeps the primary division's decision.
	if _, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: id, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &secondaryDivision, ScopeDivisionID: &secondaryDivision, Status: entity.OprecStaffStatusAccepted, AcceptedDivisionID: &subdivisions[2].ID}); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.GetByID(id, &secondaryDivision)
	if err != nil || !saved.PrimaryRejected || saved.SecondaryRejected {
		t.Fatalf("acceptance erased primary decision: %+v %v", saved, err)
	}
	audit, err = repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: id, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &secondaryDivision, ScopeDivisionID: &secondaryDivision, RejectionTarget: "secondary"})
	if err != nil || audit.PreviousStatus != entity.OprecStaffStatusAccepted || audit.NewStatus != entity.OprecStaffStatusRejected || !audit.NewPrimaryRejected || !audit.NewSecondaryRejected {
		t.Fatalf("wrong final audit: %+v %v", audit, err)
	}
	saved, err = repo.GetByID(id, nil)
	if err != nil || saved.CurrentSubdivisionID != nil || saved.AcceptedDivisiID != nil {
		t.Fatalf("rejected placement remains: %+v %v", saved, err)
	}
	if _, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: id, ActorID: actor.ID, ActorRole: entity.RoleSuperAdmin, Status: entity.OprecStaffStatusPending}); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.GetByID(id, nil)
	if err != nil || saved.PrimaryRejected || saved.SecondaryRejected {
		t.Fatalf("reopen did not clear decisions: %+v %v", saved, err)
	}
	// Both divisions can decide concurrently without overwriting each other.
	concurrentID := registrants[1].ID
	errorsCh := make(chan error, 2)
	go func() {
		_, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: concurrentID, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &secondaryDivision, ScopeDivisionID: &secondaryDivision, RejectionTarget: "primary"})
		errorsCh <- err
	}()
	go func() {
		_, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: concurrentID, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &primaryDivision, ScopeDivisionID: &primaryDivision, RejectionTarget: "secondary"})
		errorsCh <- err
	}()
	for i := 0; i < 2; i++ {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	saved, err = repo.GetByID(concurrentID, nil)
	if err != nil || saved.Status != entity.OprecStaffStatusRejected || !saved.PrimaryRejected || !saved.SecondaryRejected {
		t.Fatalf("concurrent decisions lost: %+v %v", saved, err)
	}
	if _, err := repo.UpdateStatus(repository.UpdateStatusParams{RegistrantID: registrants[2].ID, ActorID: actor.ID, ActorRole: entity.RoleDivisionAdmin, ActorDivisionID: &primaryDivision, ScopeDivisionID: &primaryDivision, RejectionTarget: "both"}); err != nil {
		t.Fatalf("same-parent reject both: %v", err)
	}
}
