package repository

import (
	"strings"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestWithDivisionScopeBuildsServerSideRestriction(t *testing.T) {
	database, err := gorm.Open(
		postgres.Open("host=127.0.0.1 user=test dbname=test sslmode=disable"),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("create dry-run database: %v", err)
	}
	divisionID := uuid.New()
	statement := withDivisionScope(database.Model(&entity.OprecStaff{}), &divisionID).
		Find(&[]entity.OprecStaff{}).Statement
	sql := statement.SQL.String()
	for _, required := range []string{"accepted_divisi_id", "current_subdivision_id", "oprec_staff_choices", "subdivisi", "s.divisi_id"} {
		if !strings.Contains(sql, required) {
			t.Errorf("scoped SQL does not contain %q: %s", required, sql)
		}
	}
	if len(statement.Vars) != 3 {
		t.Fatalf("scoped SQL variables = %d, want 3", len(statement.Vars))
	}
}

func TestWithDivisionScopeLeavesSuperadminQueryUnrestricted(t *testing.T) {
	database, err := gorm.Open(
		postgres.Open("host=127.0.0.1 user=test dbname=test sslmode=disable"),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("create dry-run database: %v", err)
	}
	statement := withDivisionScope(database.Model(&entity.OprecStaff{}), nil).
		Find(&[]entity.OprecStaff{}).Statement
	if strings.Contains(statement.SQL.String(), "oprec_staff_choices") {
		t.Fatalf("superadmin SQL is unexpectedly restricted: %s", statement.SQL.String())
	}
}

func TestOrderedChoicePreloadDoesNotHideAnotherDivisionChoice(t *testing.T) {
	database, err := gorm.Open(
		postgres.Open("host=127.0.0.1 user=test dbname=test sslmode=disable"),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("create dry-run database: %v", err)
	}

	statement := orderedChoicePreload(database.Model(&entity.OprecStaffChoice{})).
		Find(&[]entity.OprecStaffChoice{}).Statement
	sql := statement.SQL.String()
	if strings.Contains(sql, "divisi_id") || strings.Contains(sql, "subdivisi") {
		t.Fatalf("choice preload is unexpectedly restricted by division: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY pilihan ASC") {
		t.Fatalf("choice preload does not preserve choice order: %s", sql)
	}
}

func TestFilteredQueryAppliesListFiltersWithoutPagination(t *testing.T) {
	database, err := gorm.Open(
		postgres.Open("host=127.0.0.1 user=test dbname=test sslmode=disable"),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("create dry-run database: %v", err)
	}
	repository := &oprecStaffRepository{db: database}
	facultyID, subdivisionID := uuid.New(), uuid.New()
	batch := int16(2025)
	params := domain.RegistrantListParams{
		Search: "budi", Status: "interview", FacultyID: &facultyID, SubdivisionID: &subdivisionID,
		Batch: &batch, SortBy: "name", SortDirection: "asc",
	}

	sql := repository.filteredQuery(params, nil).
		Order(orderClause(params)).
		Find(&[]entity.OprecStaff{}).Statement.SQL.String()
	for _, required := range []string{"LOWER(nama) LIKE", "status =", "fakultas_id =", "angkatan =", "oprec_staff_choices", "ORDER BY nama ASC, id ASC"} {
		if !strings.Contains(sql, required) {
			t.Errorf("export SQL does not contain %q: %s", required, sql)
		}
	}
	for _, forbidden := range []string{"LIMIT", "OFFSET"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("export SQL unexpectedly contains %s: %s", forbidden, sql)
		}
	}
}

func TestOrderClauseFallsBackToCreatedAt(t *testing.T) {
	if got, want := orderClause(domain.RegistrantListParams{SortBy: "email; DROP TABLE x"}), "created_at DESC, id ASC"; got != want {
		t.Fatalf("order clause = %q, want %q", got, want)
	}
}
