package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	seeder "github.com/Petrolida/Petrolida-2027-Backend/db/seeder"
	"github.com/Petrolida/Petrolida-2027-Backend/db/seeder/seeds"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/router"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func staffFinalizationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set")
	}
	admin := openMigrationTestDB(t, "")
	schema := fmt.Sprintf("staff_finalization_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schema)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema)).Error
		closeMigrationTestDB(t, admin)
	})
	database := openMigrationTestDB(t, schema)
	t.Cleanup(func() { closeMigrationTestDB(t, database) })
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func seedStaffMasterData(t *testing.T, database *gorm.DB) {
	t.Helper()
	for _, seed := range []seeder.Seeder{seeds.NewFakultasSeeder(), seeds.NewDepartemenSeeder(), seeds.NewDivisiSeeder(), seeds.NewSubdivisiSeeder()} {
		if err := seed.Seeder(database); err != nil {
			t.Fatal(err)
		}
	}
}

type finalizedStaffAuth struct{ service.AuthService }

func (finalizedStaffAuth) Authenticate(token string) (domain.AdminProfile, error) {
	if token != string(entity.RoleSuperAdmin) && token != string(entity.RoleDivisionAdmin) {
		return domain.AdminProfile{}, domain.ErrUnauthenticated
	}
	return domain.AdminProfile{Role: token, Division: &domain.MasterReference{ID: uuid.New()}}, nil
}

func TestStaffFinalizationAndReadOnlyAPIInPostgres(t *testing.T) {
	database := staffFinalizationTestDB(t)
	t.Chdir("../..")
	seedStaffMasterData(t, database)
	rosterData, err := os.ReadFile("db/seeder/data/json/staff_results.json")
	if err != nil {
		t.Fatal(err)
	}
	var roster []struct {
		NRP           string    `json:"nrp"`
		SubdivisionID uuid.UUID `json:"subdivision_id"`
	}
	if err := json.Unmarshal(rosterData, &roster); err != nil {
		t.Fatal(err)
	}
	var department entity.Departemen
	if err := database.First(&department).Error; err != nil {
		t.Fatal(err)
	}
	statuses := []entity.OprecStaffStatus{entity.OprecStaffStatusPending, entity.OprecStaffStatusScreeningPrimary, entity.OprecStaffStatusScreeningSecondary, entity.OprecStaffStatusInterview, entity.OprecStaffStatusAccepted, entity.OprecStaffStatusRejected}
	registrants := make([]entity.OprecStaff, 0, len(roster)-1+len(statuses))
	for i := 0; i < cap(registrants); i++ {
		nrp := fmt.Sprintf("900000000%d", i-(len(roster)-1))
		if i < len(roster)-1 {
			nrp = roster[i].NRP
		}
		subdivision := roster[0].SubdivisionID
		registrants = append(registrants, entity.OprecStaff{
			Nama: fmt.Sprintf("Registered name %d", i), Email: fmt.Sprintf("staff%d@example.test", i), NRP: nrp,
			NomorTelepon: "081234567890", FakultasID: department.FakultasID, DepartemenID: department.ID, Angkatan: 2026,
			Status: statuses[i%len(statuses)], AcceptedDivisiID: &subdivision, CurrentSubdivisionID: &subdivision,
			PrimaryRejected: true, SecondaryRejected: true, TentangPetrolida: "Original registration answer",
		})
	}
	if err := database.CreateInBatches(registrants, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&entity.OprecStaffChoice{OprecStaffID: registrants[0].ID, Pilihan: 1, SubdivisiID: roster[0].SubdivisionID}).Error; err != nil {
		t.Fatal(err)
	}
	interview := entity.InterviewResult{NRP: registrants[0].NRP, Name: "Published interview name", Status: "accepted", SubdivisionID: &roster[0].SubdivisionID}
	if err := database.Create(&interview).Error; err != nil {
		t.Fatal(err)
	}
	deleted := registrants[0]
	deleted.ID, deleted.NRP, deleted.Email, deleted.Status = uuid.New(), "8000000000", "deleted@example.test", entity.OprecStaffStatusPending
	if err := database.Create(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Delete(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	// Competing startup processes must commit exactly one finalization.
	completed := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { completed <- seeds.NewStaffResultSeeder().Seeder(database) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-completed; err != nil {
			t.Fatal(err)
		}
	}
	var selection entity.StaffSelection
	if err := database.First(&selection).Error; err != nil {
		t.Fatal(err)
	}
	if selection.FinalizedAt == nil || selection.AcceptedCount != int64(len(roster)-1) || selection.RejectedCount != int64(len(statuses)) || selection.UnmatchedCount != 1 || len(selection.RosterHash) != 64 {
		t.Fatalf("finalization = %+v", selection)
	}
	for i, original := range registrants {
		var got entity.OprecStaff
		if err := database.First(&got, "id = ?", original.ID).Error; err != nil {
			t.Fatal(err)
		}
		if got.Nama != original.Nama || got.Email != original.Email || got.TentangPetrolida != original.TentangPetrolida || got.CurrentSubdivisionID != nil {
			t.Fatalf("registration data or placement changed unexpectedly: %+v", got)
		}
		if i < len(roster)-1 {
			if got.Status != entity.OprecStaffStatusAccepted || got.AcceptedDivisiID == nil || *got.AcceptedDivisiID != roster[i].SubdivisionID {
				t.Fatalf("roster member not accepted: %+v", got)
			}
			if i == 0 && (got.PrimaryRejected || !got.SecondaryRejected) {
				t.Fatal("accepted choice rejection was not cleared independently")
			}
		} else if got.Status != entity.OprecStaffStatusRejected || got.AcceptedDivisiID != nil || !got.PrimaryRejected || !got.SecondaryRejected {
			t.Fatalf("off-roster registrant not rejected: %+v", got)
		}
	}
	var gotDeleted entity.OprecStaff
	if err := database.Unscoped().First(&gotDeleted, "id = ?", deleted.ID).Error; err != nil || gotDeleted.Status != entity.OprecStaffStatusPending {
		t.Fatalf("deleted registration was modified: %+v, %v", gotDeleted, err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	cfg := config.OprecConfig{AnnouncementAt: time.Now().Add(-time.Hour), StaffGroupURL: "https://example.com/group"}
	announcements := service.NewAnnouncement(repository.NewAnnouncement(database), cfg)
	router.Announcements(engine, handler.NewAnnouncement(announcements))
	for _, tc := range []struct {
		nrp, code string
		status    int
	}{
		{roster[0].NRP, `"status":"accepted"`, 200},
		{registrants[len(roster)-1].NRP, `"status":"rejected"`, 200},
		{roster[len(roster)-1].NRP, "REGISTRANT_NOT_FOUND", 404},
		{"8000000000", "REGISTRANT_NOT_FOUND", 404},
		{"7999999999", "REGISTRANT_NOT_FOUND", 404},
	} {
		req := httptest.NewRequest("POST", "/api/v1/announcements/staff", strings.NewReader(`{"nrp":"`+tc.nrp+`"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		if res.Code != tc.status || !strings.Contains(res.Body.String(), tc.code) || res.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s lookup = %d %s", tc.nrp, res.Code, res.Body.String())
		}
		if strings.Contains(res.Body.String(), "example.test") || strings.Contains(res.Body.String(), "081234567890") {
			t.Fatal("public response leaked private registration data")
		}
	}
	oprecRepo := repository.NewOprecStaff(database)
	// A stale service config must still be rejected by the database-backed guard.
	router.OprecStaff(engine, handler.NewOprecStaff(service.NewOprecStaff(oprecRepo, cfg, nil, "")), nil, finalizedStaffAuth{}, config.Config{})
	for _, role := range []entity.UserRole{entity.RoleSuperAdmin, entity.RoleDivisionAdmin} {
		req := httptest.NewRequest("PATCH", "/api/v1/oprec-staff/"+registrants[0].ID.String()+"/status", strings.NewReader(`{"status":"pending"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+string(role))
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		if res.Code != 409 || !strings.Contains(res.Body.String(), "OPREC_SELECTION_FINALIZED") {
			t.Fatalf("%s update = %d %s", role, res.Code, res.Body.String())
		}
	}
	if err := oprecRepo.Create(&entity.OprecStaff{}, nil, nil, nil, nil); !errors.Is(err, domain.ErrSelectionFinalized) {
		t.Fatalf("registration after finalization = %v", err)
	}
	var beforeRestart []entity.OprecStaff
	if err := database.Order("nrp").Find(&beforeRestart).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	if err := seeder.RunSeeder(database); err != nil {
		t.Fatal(err)
	}
	var afterRestart []entity.OprecStaff
	if err := database.Order("nrp").Find(&afterRestart).Error; err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(beforeRestart)
	after, _ := json.Marshal(afterRestart)
	if string(before) != string(after) {
		t.Fatal("restart rewrote finalized registrations")
	}
	var gotSelection entity.StaffSelection
	if err := database.First(&gotSelection).Error; err != nil || *gotSelection.FinalizedAt != *selection.FinalizedAt || gotSelection.RosterHash != selection.RosterHash {
		t.Fatalf("restart changed finalization: %+v, %v", gotSelection, err)
	}
	result, err := repository.NewAnnouncement(database).GetInterviewResult(context.Background(), interview.NRP)
	if err != nil || result.Name != interview.Name || result.Status != interview.Status {
		t.Fatalf("finalization changed interview results: %+v, %v", result, err)
	}
}

func TestStaffFinalizationFailureRollsBackInPostgres(t *testing.T) {
	database := staffFinalizationTestDB(t)
	t.Chdir("../..")
	faculty := entity.Fakultas{Nama: "Fixture"}
	if err := database.Create(&faculty).Error; err != nil {
		t.Fatal(err)
	}
	department := entity.Departemen{Nama: "Fixture", FakultasID: faculty.ID}
	if err := database.Create(&department).Error; err != nil {
		t.Fatal(err)
	}
	rows := []entity.OprecStaff{
		{Nama: "Accepted fixture", Email: "accepted@example.test", NRP: "5050261007", NomorTelepon: "081234567890", FakultasID: faculty.ID, DepartemenID: department.ID, Status: entity.OprecStaffStatusPending},
		{Nama: "Rejected fixture", Email: "rejected@example.test", NRP: "9000000000", NomorTelepon: "081234567890", FakultasID: faculty.ID, DepartemenID: department.ID, Status: entity.OprecStaffStatusInterview},
	}
	if err := database.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	// The roster's subdivision is intentionally missing, causing an FK failure.
	if err := seeds.NewStaffResultSeeder().Seeder(database); err == nil {
		t.Fatal("invalid final placement committed")
	}
	var selection entity.StaffSelection
	if err := database.First(&selection).Error; err != nil || selection.FinalizedAt != nil {
		t.Fatalf("failed finalization locked the selection: %+v, %v", selection, err)
	}
	for _, original := range rows {
		var got entity.OprecStaff
		if err := database.First(&got, "id = ?", original.ID).Error; err != nil || got.Status != original.Status || got.AcceptedDivisiID != nil {
			t.Fatalf("partial finalization survived rollback: %+v, %v", got, err)
		}
	}
}
