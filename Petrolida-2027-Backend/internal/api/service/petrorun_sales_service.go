package service

import (
	"context"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
)

func (s *petroRunService) SaleAvailability(ctx context.Context) (domain.PetroRunSaleAvailability, error) {
	result := domain.PetroRunSaleAvailability{RegistrationStatus: "open", Phase: "normal"}
	if s.cfg.Sales == nil {
		return result, nil
	}
	now := s.now().UTC()
	rules := *s.cfg.Sales
	used, err := s.repository.CountEarlyBirdReservations(ctx, now)
	if err != nil {
		return result, err
	}
	result.RegistrationStatus = rules.RegistrationStatus(now)
	result.EarlyBirdRemaining = max(0, rules.EarlyBirdQuota-used)
	if !now.Before(rules.LateStartAt) {
		result.Phase = "late"
	} else if result.EarlyBirdRemaining > 0 {
		result.Phase = "early_bird"
	}
	return result, nil
}

func petroRunPriceLines(order entity.PetroRunOrder) []domain.PetroRunPriceLine {
	if order.BaseSubtotalIDR == 0 {
		return nil
	}
	lines := make([]domain.PetroRunPriceLine, 0, 2)
	for _, tier := range []string{"early_bird", "normal", "late"} {
		line := domain.PetroRunPriceLine{Tier: tier}
		for _, p := range order.Participants {
			if p.PriceTier == tier {
				line.Quantity++
				line.UnitPriceIDR = p.BasePriceIDR
				line.SubtotalIDR += p.BasePriceIDR
			}
		}
		if line.Quantity > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}
