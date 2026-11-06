package repository

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/google/uuid"
)

// Resolve while holding the registrant row lock so concurrent decisions cannot
// erase a rejection made by the other division.
func resolveRejection(registrant entity.OprecStaff, choices []entity.OprecStaffChoice, divisionID *uuid.UUID, target string) (entity.OprecStaffStatus, *uuid.UUID, bool, bool, error) {
	invalid := func(message string) (entity.OprecStaffStatus, *uuid.UUID, bool, bool, error) {
		return "", nil, false, false, myerror.NewFieldValidation("rejection_target", "Rejection", message)
	}
	if target != "primary" && target != "secondary" && target != "both" {
		return invalid("must be primary, secondary, or both")
	}
	byPriority := map[int16]entity.OprecStaffChoice{}
	for _, choice := range choices {
		byPriority[choice.Pilihan] = choice
	}
	for _, priority := range []int16{1, 2} {
		if target != "both" && ((target == "primary") != (priority == 1)) {
			continue
		}
		choice, ok := byPriority[priority]
		if !ok || choice.Subdivisi == nil || choice.Subdivisi.Divisi == nil {
			return invalid("division choice is unavailable")
		}
		if divisionID != nil && choice.Subdivisi.DivisiID != *divisionID {
			return "", nil, false, false, domain.ErrForbidden
		}
	}
	primary, secondary := registrant.PrimaryRejected, registrant.SecondaryRejected
	// Older records may predate the per-choice flags.
	primary = primary || registrant.Status == entity.OprecStaffStatusScreeningSecondary || registrant.Status == entity.OprecStaffStatusRejected
	secondary = secondary || registrant.Status == entity.OprecStaffStatusRejected
	primary = primary || target == "primary" || target == "both"
	secondary = secondary || target == "secondary" || target == "both"
	if primary && secondary {
		return entity.OprecStaffStatusRejected, nil, true, true, nil
	}
	priority, status := int16(1), entity.OprecStaffStatusScreeningPrimary
	if primary {
		priority, status = 2, entity.OprecStaffStatusScreeningSecondary
	}
	choice, ok := byPriority[priority]
	if !ok || choice.Subdivisi == nil || !choice.Subdivisi.IsActive || choice.Subdivisi.Divisi == nil || !choice.Subdivisi.Divisi.IsActive {
		return invalid("remaining division choice must be active")
	}
	id := choice.SubdivisiID
	return status, &id, primary, secondary, nil
}
