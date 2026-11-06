package config

import "testing"

func TestPetroRunSaleDefaultsDoNotActivateUnconfiguredCheckout(t *testing.T) {
	for _, key := range append(petroRunSaleEnvKeys, "PETRORUN_BASE_PRICE_IDR", "PETRORUN_ADMIN_FEE_IDR", "PETRORUN_BANK_NAME", "PETRORUN_BANK_ACCOUNT_NUMBER", "PETRORUN_BANK_ACCOUNT_HOLDER", "PETRORUN_ACCESS_SECRET", "PETRORUN_JERSEY_SURCHARGE_2XL_IDR", "PETRORUN_JERSEY_SURCHARGE_3XL_IDR", "PETRORUN_JERSEY_SURCHARGE_4XL_IDR") {
		t.Setenv(key, "")
	}
	cfg, err := loadPetroRunConfig()
	if err != nil || cfg.Available() {
		t.Fatalf("unconfigured checkout: %+v %v", cfg, err)
	}
	r := cfg.Sales
	if r.EarlyBirdPriceIDR != 125_000 || r.EarlyBirdQuota != 25 || r.LatePriceIDR != 139_000 || r.RegistrationStartAt.Format("2006-01-02T15:04:05Z07:00") != "2026-10-24T00:00:00+07:00" || r.RegistrationEndAt.Format("2006-01-02T15:04:05Z07:00") != "2026-11-23T00:00:00+07:00" || r.LateStartAt.Day() != 19 || r.RacepackAt.Day() != 25 {
		t.Fatalf("wrong official schedule: %+v", r)
	}
}

func TestPetroRunSaleConfigurationRejectsInvalidOverrides(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{"PETRORUN_EARLY_BIRD_QUOTA", "-1"},
		{"PETRORUN_EARLY_BIRD_PRICE_IDR", "160000"},
		{"PETRORUN_LATE_PRICE_IDR", "-1"},
		{"PETRORUN_REGISTRATION_START_AT", "2026-10-24"},
		{"PETRORUN_LATE_START_AT", "2026-11-23T00:00:00+07:00"},
		{"PETRORUN_RACEPACK_AT", "2026-11-01T00:00:00+07:00"},
	} {
		t.Run(test.key, func(t *testing.T) {
			for _, key := range petroRunSaleEnvKeys {
				t.Setenv(key, "")
			}
			t.Setenv(test.key, test.value)
			rules, err := loadPetroRunSales(150_000)
			if err == nil {
				err = validatePetroRunSales(*rules)
			}
			if err == nil {
				t.Fatalf("accepted %s=%s", test.key, test.value)
			}
		})
	}
}
