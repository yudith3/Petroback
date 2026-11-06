package repository

import (
	"errors"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func TestResolveRejection(t *testing.T) {
	primaryDivision, secondaryDivision := uuid.New(), uuid.New()
	primaryID, secondaryID := uuid.New(), uuid.New()
	choices := []entity.OprecStaffChoice{
		{Pilihan: 1, SubdivisiID: primaryID, Subdivisi: &entity.Subdivisi{ID: primaryID, DivisiID: primaryDivision, IsActive: true, Divisi: &entity.Divisi{ID: primaryDivision, IsActive: true}}},
		{Pilihan: 2, SubdivisiID: secondaryID, Subdivisi: &entity.Subdivisi{ID: secondaryID, DivisiID: secondaryDivision, IsActive: true, Divisi: &entity.Divisi{ID: secondaryDivision, IsActive: true}}},
	}
	tests := []struct {
		name, target       string
		registrant         entity.OprecStaff
		scope              *uuid.UUID
		status             entity.OprecStaffStatus
		current            *uuid.UUID
		primary, secondary bool
		forbidden          bool
	}{
		{name: "primary handoff", target: "primary", scope: &primaryDivision, status: entity.OprecStaffStatusScreeningSecondary, current: &secondaryID, primary: true},
		{name: "secondary first", target: "secondary", scope: &secondaryDivision, status: entity.OprecStaffStatusScreeningPrimary, current: &primaryID, secondary: true},
		{name: "secondary after primary", target: "secondary", registrant: entity.OprecStaff{PrimaryRejected: true}, scope: &secondaryDivision, status: entity.OprecStaffStatusRejected, primary: true, secondary: true},
		{name: "primary after secondary", target: "primary", registrant: entity.OprecStaff{SecondaryRejected: true}, status: entity.OprecStaffStatusRejected, primary: true, secondary: true},
		{name: "both", target: "both", status: entity.OprecStaffStatusRejected, primary: true, secondary: true},
		{name: "legacy secondary screening", target: "secondary", registrant: entity.OprecStaff{Status: entity.OprecStaffStatusScreeningSecondary}, status: entity.OprecStaffStatusRejected, primary: true, secondary: true},
		{name: "wrong primary owner", target: "primary", scope: &secondaryDivision, forbidden: true},
		{name: "both needs both owners", target: "both", scope: &primaryDivision, forbidden: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, current, primary, secondary, err := resolveRejection(test.registrant, choices, test.scope, test.target)
			if test.forbidden {
				if !errors.Is(err, domain.ErrForbidden) {
					t.Fatalf("error = %v, want forbidden", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if status != test.status || primary != test.primary || secondary != test.secondary {
				t.Fatalf("decision = %s/%v/%v", status, primary, secondary)
			}
			if (current == nil) != (test.current == nil) || (current != nil && *current != *test.current) {
				t.Fatalf("current = %v, want %v", current, test.current)
			}
		})
	}
	t.Run("inactive destination", func(t *testing.T) {
		choices[1].Subdivisi.IsActive = false
		if _, _, _, _, err := resolveRejection(entity.OprecStaff{}, choices, nil, "primary"); err == nil {
			t.Fatal("inactive destination accepted")
		}
	})
}
