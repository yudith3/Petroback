package migrations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func TestSubmissionReceiptIsAtomicAndUnique(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set; PostgreSQL confirmation test runs in CI")
	}
	admin := openMigrationTestDB(t, "")
	schema := fmt.Sprintf("confirmation_test_%d", time.Now().UnixNano())
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
	faculty := entity.Fakultas{Nama: "Fixture faculty"}
	if err := database.Create(&faculty).Error; err != nil {
		t.Fatal(err)
	}
	department := entity.Departemen{Nama: "Fixture department", FakultasID: faculty.ID}
	if err := database.Create(&department).Error; err != nil {
		t.Fatal(err)
	}
	division := entity.Divisi{Nama: "Fixture division"}
	if err := database.Create(&division).Error; err != nil {
		t.Fatal(err)
	}
	subdivisions := []entity.Subdivisi{{Nama: "Fixture primary", DivisiID: division.ID}, {Nama: "Fixture secondary", DivisiID: division.ID}}
	if err := database.Create(&subdivisions).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewOprecStaff(database)
	fingerprint := strings.Repeat("a", 64)
	create := func(index int, key *uuid.UUID, invalid bool) (entity.OprecStaff, error) {
		registrant := entity.OprecStaff{SubmissionKey: key, SubmissionFingerprint: &fingerprint, Nama: "Fixture applicant", Email: fmt.Sprintf("fixture%d@example.test", index), NRP: fmt.Sprintf("000000%04d", index), NomorTelepon: "+6281234567890", FakultasID: faculty.ID, DepartemenID: department.ID, Angkatan: 2026, TentangPetrolida: "Fixture", Status: entity.OprecStaffStatusPending}
		choices := []entity.OprecStaffChoice{{Pilihan: 1, SubdivisiID: subdivisions[0].ID}, {Pilihan: 2, SubdivisiID: subdivisions[1].ID}}
		if invalid {
			choices[0].SubdivisiID = uuid.New()
		}
		err := repo.Create(&registrant, choices, &entity.OprecStaffSkill{}, &entity.OprecStaffAdditionalInformation{}, []entity.OprecStaffDocument{{Tipe: "required_documents", URL: "https://example.test/fixture"}})
		return registrant, err
	}
	key := uuid.New()
	if _, err := create(1, &key, true); err == nil {
		t.Fatal("invalid related row unexpectedly committed")
	}
	if _, err := repo.GetBySubmissionKey(context.Background(), key); !errors.Is(err, domain.ErrRegistrantNotFound) {
		t.Fatalf("rolled-back receipt was visible: %v", err)
	}
	saved, err := create(1, &key, false)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := repo.GetBySubmissionKey(context.Background(), key)
	if err != nil || receipt.ID != saved.ID || receipt.Nama != "" || receipt.Email != "" {
		t.Fatalf("receipt exposed fields or lost commit: %#v %v", receipt, err)
	}
	if _, err := create(2, &key, false); !errors.Is(err, domain.ErrRegistrationExists) {
		t.Fatalf("duplicate submission key accepted: %v", err)
	}
	// Legacy clients can continue registering with no receipt key.
	for index := 3; index <= 4; index++ {
		if _, err := create(index, nil, false); err != nil {
			t.Fatal(err)
		}
	}
	var choices, documents, skills, additional int64
	for _, item := range []struct {
		table string
		count *int64
	}{{"oprec_staff_choices", &choices}, {"oprec_staff_documents", &documents}, {"oprec_staff_skills", &skills}, {"oprec_staff_additional_information", &additional}} {
		if err := database.Table(item.table).Where("oprec_staff_id = ?", saved.ID).Count(item.count).Error; err != nil {
			t.Fatal(err)
		}
	}
	if choices != 2 || documents != 1 || skills != 1 || additional != 1 {
		t.Fatalf("incomplete committed registration: %d %d %d %d", choices, documents, skills, additional)
	}
	concurrentKey := uuid.New()
	var wait sync.WaitGroup
	errorsFound := make(chan error, 2)
	for index := 5; index <= 6; index++ {
		wait.Add(1)
		go func(index int) { defer wait.Done(); _, err := create(index, &concurrentKey, false); errorsFound <- err }(index)
	}
	wait.Wait()
	close(errorsFound)
	successes, conflicts := 0, 0
	for err := range errorsFound {
		if err == nil {
			successes++
		} else if errors.Is(err, domain.ErrRegistrationExists) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent submissions: successes=%d conflicts=%d", successes, conflicts)
	}
}
