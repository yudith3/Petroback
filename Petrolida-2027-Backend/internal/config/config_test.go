package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	t.Setenv("APP_HOST", "0.0.0.0")
	t.Setenv("APP_PORT", "9000")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://petrolida.test")
	t.Setenv("DB_PASS", "password with spaces")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.HTTP.Address(), "0.0.0.0:9000"; got != want {
		t.Fatalf("HTTP address = %q, want %q", got, want)
	}

	if got, want := len(cfg.HTTP.AllowedOrigins), 2; got != want {
		t.Fatalf("allowed origins length = %d, want %d", got, want)
	}

	if databaseURL := cfg.Database.URL(); !strings.Contains(databaseURL, "password%20with%20spaces") {
		t.Fatalf("database URL does not escape credentials: %s", databaseURL)
	}
}

func TestLoadDefaultAllowedBatchesExcludes2023(t *testing.T) {
	t.Setenv("OPREC_ALLOWED_BATCHES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []int16{2024, 2025, 2026}
	if !slices.Equal(cfg.Oprec.AllowedBatches, want) {
		t.Fatalf("allowed batches = %v, want %v", cfg.Oprec.AllowedBatches, want)
	}
}

func TestLoadParsesAnnouncementAt(t *testing.T) {
	const value = "2026-11-01T12:30:00+07:00"
	t.Setenv("OPREC_ANNOUNCEMENT_AT", value)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Oprec.AnnouncementAt.Equal(want) {
		t.Fatalf("announcement at = %v, want %v", cfg.Oprec.AnnouncementAt, want)
	}
}

func TestLoadRejectsInvalidAnnouncementAt(t *testing.T) {
	t.Setenv("OPREC_ANNOUNCEMENT_AT", "next Sunday")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid announcement timestamp error")
	}
}

func TestRegistrationOpen(t *testing.T) {
	start := time.Date(2026, time.September, 23, 18, 0, 0, 0, time.FixedZone("WIB", 7*60*60))
	end := time.Date(2026, time.October, 6, 18, 0, 0, 0, time.FixedZone("WIB", 7*60*60))
	scheduled := OprecConfig{RegistrationStartAt: start, RegistrationEndAt: end}

	tests := []struct {
		name string
		cfg  OprecConfig
		now  time.Time
		want bool
	}{
		{name: "Unconfigured", cfg: OprecConfig{}, now: start, want: false},
		{name: "Missing end", cfg: OprecConfig{RegistrationStartAt: start}, now: start, want: false},
		{name: "Before start", cfg: scheduled, now: start.Add(-time.Nanosecond), want: false},
		{name: "At start", cfg: scheduled, now: start, want: true},
		{name: "Finalized", cfg: OprecConfig{SelectionFinalized: true, RegistrationStartAt: start, RegistrationEndAt: end}, now: start, want: false},
		{name: "Before end", cfg: scheduled, now: end.Add(-time.Nanosecond), want: true},
		{name: "At end", cfg: scheduled, now: end, want: false},
		{name: "After end", cfg: scheduled, now: end.Add(time.Nanosecond), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.cfg.RegistrationOpen(test.now); got != test.want {
				t.Fatalf("RegistrationOpen(%v) = %v, want %v", test.now, got, test.want)
			}
		})
	}
}

func TestLoadRejectsIncompleteRegistrationSchedule(t *testing.T) {
	t.Setenv("OPREC_REGISTRATION_START_AT", "2026-09-23T18:00:00+07:00")
	t.Setenv("OPREC_REGISTRATION_END_AT", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want incomplete registration schedule error")
	}
}

func TestLoadRejectsInvalidRegistrationRange(t *testing.T) {
	t.Setenv("OPREC_REGISTRATION_START_AT", "2026-10-06T18:00:00+07:00")
	t.Setenv("OPREC_REGISTRATION_END_AT", "2026-09-23T18:00:00+07:00")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid registration range error")
	}
}

func TestLoadPetroRunJerseySurchargesFromEnvironment(t *testing.T) {
	t.Setenv("PETRORUN_BASE_PRICE_IDR", "150000")
	t.Setenv("PETRORUN_ADMIN_FEE_IDR", "2500")
	t.Setenv("PETRORUN_JERSEY_SURCHARGE_2XL_IDR", "5000")
	t.Setenv("PETRORUN_JERSEY_SURCHARGE_3XL_IDR", "10000")
	t.Setenv("PETRORUN_JERSEY_SURCHARGE_4XL_IDR", "15000")
	t.Setenv("PETRORUN_BANK_NAME", "BNI")
	t.Setenv("PETRORUN_BANK_ACCOUNT_NUMBER", "1234567890")
	t.Setenv("PETRORUN_BANK_ACCOUNT_HOLDER", "Petrolida ITS")
	t.Setenv("PETRORUN_ACCESS_SECRET", "test-secret-at-least-32-characters-long")

	cfg, err := loadPetroRunConfig()
	if err != nil {
		t.Fatalf("loadPetroRunConfig() error = %v", err)
	}

	for size, want := range map[string]int64{
		"S": 0, "M": 0, "L": 0, "XL": 0,
		"2XL": 5_000, "3XL": 10_000, "4XL": 15_000,
	} {
		got, ok := cfg.JerseySize(size)
		if !ok || got.SurchargeIDR != want {
			t.Errorf("JerseySize(%q) = %+v, %v; want surcharge %d", size, got, ok, want)
		}
	}
}

func TestLoadPetroRunRejectsNegativeJerseySurcharge(t *testing.T) {
	t.Setenv("PETRORUN_BASE_PRICE_IDR", "150000")
	t.Setenv("PETRORUN_ADMIN_FEE_IDR", "2500")
	t.Setenv("PETRORUN_JERSEY_SURCHARGE_3XL_IDR", "-1")
	t.Setenv("PETRORUN_BANK_NAME", "BNI")
	t.Setenv("PETRORUN_BANK_ACCOUNT_NUMBER", "1234567890")
	t.Setenv("PETRORUN_BANK_ACCOUNT_HOLDER", "Petrolida ITS")
	t.Setenv("PETRORUN_ACCESS_SECRET", "test-secret-at-least-32-characters-long")

	if _, err := loadPetroRunConfig(); err == nil {
		t.Fatal("loadPetroRunConfig() error = nil, want negative surcharge validation error")
	}
}

func TestAnnouncementOpen(t *testing.T) {
	announcementAt := time.Date(2026, time.November, 1, 12, 30, 0, 0, time.FixedZone("WIB", 7*60*60))

	tests := []struct {
		name string
		cfg  OprecConfig
		now  time.Time
		want bool
	}{
		{name: "Unconfigured", cfg: OprecConfig{}, now: announcementAt, want: false},
		{name: "Before schedule", cfg: OprecConfig{AnnouncementAt: announcementAt}, now: announcementAt.Add(-time.Nanosecond), want: false},
		{name: "At schedule", cfg: OprecConfig{AnnouncementAt: announcementAt}, now: announcementAt, want: true},
		{name: "After schedule", cfg: OprecConfig{AnnouncementAt: announcementAt}, now: announcementAt.Add(time.Nanosecond), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.cfg.AnnouncementOpen(test.now); got != test.want {
				t.Fatalf("AnnouncementOpen(%v) = %v, want %v", test.now, got, test.want)
			}
		})
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("HTTP_READ_TIMEOUT", "eventually")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}

func TestLoadRejectsWeakProductionSecret(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("JWT_SECRET", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want weak production JWT secret error")
	}
}

func TestLoadRejectsNonCanonicalSuperadminEmail(t *testing.T) {
	t.Setenv("ADMIN_EMAIL", "another-admin@petrolida.com")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want canonical superadmin email error")
	}
}

func TestLoadRequiresMultipartOverheadRoom(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("OPREC_PORTFOLIO_MAX_BYTES", "1024")
	t.Setenv("REQUEST_BODY_LIMIT_BYTES", "1024")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want request body limit error")
	}
}

func TestLoadRejectsIncompleteDriveConfig(t *testing.T) {
	t.Setenv("PORTFOLIO_STORAGE_DRIVER", "gdrive")
	t.Setenv("GDRIVE_CLIENT_ID", "client-id")
	t.Setenv("GDRIVE_CLIENT_SECRET", "client-secret")
	t.Setenv("GDRIVE_REFRESH_TOKEN", "")
	t.Setenv("GDRIVE_FOLDER_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() menerima konfigurasi gdrive yang belum lengkap")
	}
	for _, want := range []string{"GDRIVE_REFRESH_TOKEN", "GDRIVE_FOLDER_ID"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("pesan error tidak menyebut %s: %v", want, err)
		}
	}
}

func TestLoadRejectsUnknownStorageDriver(t *testing.T) {
	t.Setenv("PORTFOLIO_STORAGE_DRIVER", "dropbox")

	if _, err := Load(); err == nil {
		t.Fatal("Load() menerima driver penyimpanan yang tidak dikenal")
	}
}

func TestLoadDefaultsToLocalStorage(t *testing.T) {
	t.Setenv("PORTFOLIO_STORAGE_DRIVER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Storage.Driver != StorageDriverLocal {
		t.Fatalf("Driver = %q, want %q", cfg.Storage.Driver, StorageDriverLocal)
	}
}

func TestLoadParsesPublicURL(t *testing.T) {
	t.Setenv("APP_PUBLIC_URL", "https://petrolida.com/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.App.PublicURL, "https://petrolida.com"; got != want {
		t.Fatalf("public URL = %q, want %q", got, want)
	}
}

func TestLoadRejectsInvalidPublicURL(t *testing.T) {
	for _, value := range []string{
		"petrolida.com",
		"ftp://petrolida.com",
		"https://petrolida.com/admin",
		"https://petrolida.com?x=1",
		"https://user@petrolida.com",
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("APP_PUBLIC_URL", value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted APP_PUBLIC_URL %q", value)
			}
		})
	}
}
