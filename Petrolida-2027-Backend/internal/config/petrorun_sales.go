package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
)

var petroRunSaleEnvKeys = []string{
	"PETRORUN_EARLY_BIRD_PRICE_IDR", "PETRORUN_EARLY_BIRD_QUOTA", "PETRORUN_LATE_PRICE_IDR",
	"PETRORUN_REGISTRATION_START_AT", "PETRORUN_REGISTRATION_END_AT", "PETRORUN_LATE_START_AT", "PETRORUN_RACEPACK_AT",
}

// Defaults are the organizer's 19 October 2026 schedule. Bank details and fees
// remain environment configuration; these defaults do not activate checkout.
func loadPetroRunSales(normalPrice int64) (*domain.PetroRunSaleRules, error) {
	early, err := int64FromEnv("PETRORUN_EARLY_BIRD_PRICE_IDR", 125_000)
	if err != nil {
		return nil, err
	}
	quota, err := int64FromEnv("PETRORUN_EARLY_BIRD_QUOTA", 25)
	if err != nil {
		return nil, err
	}
	if quota < 1 || quota > 100_000 {
		return nil, fmt.Errorf("PETRORUN_EARLY_BIRD_QUOTA must be between 1 and 100000")
	}
	late, err := int64FromEnv("PETRORUN_LATE_PRICE_IDR", 139_000)
	if err != nil {
		return nil, err
	}
	rules := &domain.PetroRunSaleRules{NormalPriceIDR: normalPrice, EarlyBirdPriceIDR: early, EarlyBirdQuota: int(quota), LatePriceIDR: late}
	for _, item := range []struct {
		key, fallback string
		target        *time.Time
	}{
		{"PETRORUN_REGISTRATION_START_AT", "2026-10-24T00:00:00+07:00", &rules.RegistrationStartAt},
		{"PETRORUN_REGISTRATION_END_AT", "2026-11-23T00:00:00+07:00", &rules.RegistrationEndAt},
		// H-3 relative to the final registration date, 22 November.
		{"PETRORUN_LATE_START_AT", "2026-11-19T00:00:00+07:00", &rules.LateStartAt},
		{"PETRORUN_RACEPACK_AT", "2026-11-25T00:00:00+07:00", &rules.RacepackAt},
	} {
		value := strings.TrimSpace(os.Getenv(item.key))
		if value == "" {
			value = item.fallback
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, fmt.Errorf("%s must be an RFC3339 timestamp with timezone: %w", item.key, err)
		}
		*item.target = parsed
	}
	return rules, nil
}

func validatePetroRunSales(r domain.PetroRunSaleRules) error {
	if r.EarlyBirdPriceIDR <= 0 || r.LatePriceIDR <= 0 || r.EarlyBirdPriceIDR > r.NormalPriceIDR || r.LatePriceIDR > r.NormalPriceIDR {
		return fmt.Errorf("PETRORUN early bird and late prices must be positive and no higher than the normal price")
	}
	if r.EarlyBirdQuota < 1 || r.EarlyBirdQuota > 100_000 {
		return fmt.Errorf("PETRORUN early bird quota must be between 1 and 100000")
	}
	if !r.RegistrationStartAt.Before(r.LateStartAt) || !r.LateStartAt.Before(r.RegistrationEndAt) || r.RacepackAt.Before(r.RegistrationEndAt) {
		return fmt.Errorf("PETRORUN schedule must satisfy registration start < late start < registration end <= racepack date")
	}
	return nil
}
