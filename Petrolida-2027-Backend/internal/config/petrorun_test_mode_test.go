package config

import "testing"

func TestPetroRunTestModeRequiresStagingIdentity(t *testing.T) {
	cfg := PetroRunConfig{TestMode: true, BasePriceIDR: 150000, BankName: "test", BankAccountNumber: "0000000000", BankAccountHolder: "test", AccessSecret: "staging-test-secret-at-least-32-characters"}
	t.Setenv("APP_NAME", "petrolida-2027-api")
	if err := cfg.Validate(); err == nil {
		t.Fatal("production accepted test mode")
	}
	t.Setenv("APP_NAME", "petrolida-staging-api")
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPetroRunFalseTestFlagDoesNotActivateMissingPaymentConfiguration(t *testing.T) {
	for _, key := range append(petroRunSaleEnvKeys, "PETRORUN_BASE_PRICE_IDR", "PETRORUN_ADMIN_FEE_IDR", "PETRORUN_BANK_NAME", "PETRORUN_BANK_ACCOUNT_NUMBER", "PETRORUN_BANK_ACCOUNT_HOLDER", "PETRORUN_ACCESS_SECRET", "PETRORUN_JERSEY_SURCHARGE_2XL_IDR", "PETRORUN_JERSEY_SURCHARGE_3XL_IDR", "PETRORUN_JERSEY_SURCHARGE_4XL_IDR") {
		t.Setenv(key, "")
	}
	t.Setenv("PETRORUN_TEST_MODE", "false")
	cfg, err := loadPetroRunConfig()
	if err != nil || cfg.Available() || cfg.TestMode {
		t.Fatalf("disabled test flag activated checkout: %+v %v", cfg, err)
	}
}
