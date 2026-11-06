package service

import (
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func TestAdminScopeAllowsSuperadminToSeeAllDivisions(t *testing.T) {
	scope, err := adminScope(domain.AdminProfile{Role: string(entity.RoleSuperAdmin)})
	if err != nil {
		t.Fatalf("adminScope() error = %v", err)
	}
	if scope != nil {
		t.Fatalf("adminScope() = %v, want unrestricted scope", scope)
	}
}

func TestAdminScopeRestrictsDivisionAdmin(t *testing.T) {
	divisionID := uuid.New()
	scope, err := adminScope(domain.AdminProfile{
		Role:     string(entity.RoleDivisionAdmin),
		Division: &domain.MasterReference{ID: divisionID, Name: "Creative"},
	})
	if err != nil {
		t.Fatalf("adminScope() error = %v", err)
	}
	if scope == nil || *scope != divisionID {
		t.Fatalf("adminScope() = %v, want %v", scope, divisionID)
	}
}

func TestAdminScopeRejectsDivisionAdminWithoutDivision(t *testing.T) {
	_, err := adminScope(domain.AdminProfile{Role: string(entity.RoleDivisionAdmin)})
	if err == nil || err.Error() != domain.ErrForbidden.Error() {
		t.Fatalf("adminScope() error = %v, want ErrForbidden", err)
	}
}
