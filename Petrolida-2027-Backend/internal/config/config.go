package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Oprec    OprecConfig
	Storage  StorageConfig
	PetroRun PetroRunConfig
}

type AppConfig struct {
	Name        string
	Environment string
	LogLevel    string
	// PublicURL adalah origin situs (misal https://petrolida.com) untuk membuat
	// tautan absolut di export, seperti link portfolio yang butuh sesi admin.
	PublicURL string
}

type HTTPConfig struct {
	Host             string
	Port             string
	AllowedOrigins   []string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
	RequestBodyLimit int64
}

func (c HTTPConfig) Address() string {
	return net.JoinHostPort(c.Host, c.Port)
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	Timezone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type AuthConfig struct {
	JWTSecret      string
	SessionTTL     time.Duration
	CookieName     string
	CookieSecure   bool
	AdminName      string
	AdminEmail     string
	AdminPassword  string
	AdminInstansi  string
	AdminNoTelepon string
}

type OprecConfig struct {
	// Loaded from the persistent database lock after startup finalization.
	SelectionFinalized      bool
	RegistrationStartAt     time.Time
	RegistrationEndAt       time.Time
	AnnouncementAt          time.Time
	InterviewAnnouncementAt time.Time
	StaffGroupURL           string
	InterviewScheduleURL    string
	AllowedBatches          []int16
	PortfolioMaxBytes       int64
	PortfolioRequired       []string
}

func (c OprecConfig) RegistrationOpen(now time.Time) bool {
	if c.SelectionFinalized {
		return false
	}
	if c.RegistrationStartAt.IsZero() || c.RegistrationEndAt.IsZero() {
		return false
	}
	return !now.Before(c.RegistrationStartAt) && now.Before(c.RegistrationEndAt)
}

func (c OprecConfig) validate() error {
	for key, value := range map[string]string{"OPREC_STAFF_GROUP_URL": c.StaffGroupURL, "OPREC_INTERVIEW_SCHEDULE_URL": c.InterviewScheduleURL} {
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
			return fmt.Errorf("%s must be an absolute HTTPS URL", key)
		}
	}
	hasStart := !c.RegistrationStartAt.IsZero()
	hasEnd := !c.RegistrationEndAt.IsZero()
	if hasStart != hasEnd {
		return fmt.Errorf(
			"OPREC_REGISTRATION_START_AT and OPREC_REGISTRATION_END_AT must both be set or both be empty",
		)
	}
	if hasStart && !c.RegistrationStartAt.Before(c.RegistrationEndAt) {
		return fmt.Errorf("OPREC_REGISTRATION_END_AT must be after OPREC_REGISTRATION_START_AT")
	}
	return nil
}

func (c OprecConfig) AnnouncementOpen(now time.Time) bool {
	return !c.AnnouncementAt.IsZero() && !now.Before(c.AnnouncementAt)
}

// Driver penyimpanan portofolio. Lokal dipakai saat development supaya Drive
// panitia tidak terisi berkas percobaan, gdrive dipakai di produksi.
const (
	StorageDriverLocal = "local"
	StorageDriverDrive = "gdrive"
)

type StorageConfig struct {
	Driver             string
	PortfolioDirectory string
	Drive              DriveConfig
}

type DriveConfig struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	FolderID     string
	Timeout      time.Duration
}

// validate memastikan kesalahan konfigurasi ketahuan saat server dinyalakan,
// bukan saat pendaftar pertama menekan tombol kirim.
func (c StorageConfig) validate() error {
	switch c.Driver {
	case StorageDriverLocal:
		return nil
	case StorageDriverDrive:
		missing := make([]string, 0, 4)
		if c.Drive.ClientID == "" {
			missing = append(missing, "GDRIVE_CLIENT_ID")
		}
		if c.Drive.ClientSecret == "" {
			missing = append(missing, "GDRIVE_CLIENT_SECRET")
		}
		if c.Drive.RefreshToken == "" {
			missing = append(missing, "GDRIVE_REFRESH_TOKEN")
		}
		if c.Drive.FolderID == "" {
			missing = append(missing, "GDRIVE_FOLDER_ID")
		}
		if len(missing) > 0 {
			return fmt.Errorf(
				"%s is required when PORTFOLIO_STORAGE_DRIVER=%s",
				strings.Join(missing, ", "), StorageDriverDrive,
			)
		}
		return nil
	default:
		return fmt.Errorf(
			"PORTFOLIO_STORAGE_DRIVER must be %q or %q, got %q",
			StorageDriverLocal, StorageDriverDrive, c.Driver,
		)
	}
}

func (c DatabaseConfig) URL() string {
	query := url.Values{}
	query.Set("sslmode", c.SSLMode)
	query.Set("TimeZone", c.Timezone)

	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, c.Port),
		Path:     c.Name,
		RawQuery: query.Encode(),
	}).String()
}

func Load() (Config, error) {
	_ = godotenv.Load()

	readTimeout, err := durationFromEnv("HTTP_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	writeTimeout, err := durationFromEnv("HTTP_WRITE_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	idleTimeout, err := durationFromEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := durationFromEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	connectionLifetime, err := durationFromEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}

	sessionTTL, err := durationFromEnv("AUTH_SESSION_TTL", 8*time.Hour)
	if err != nil {
		return Config{}, err
	}

	registrationStartAt, err := optionalTimeFromEnv("OPREC_REGISTRATION_START_AT")
	if err != nil {
		return Config{}, err
	}

	registrationEndAt, err := optionalTimeFromEnv("OPREC_REGISTRATION_END_AT")
	if err != nil {
		return Config{}, err
	}

	interviewAnnouncementAt, err := optionalTimeFromEnv("OPREC_INTERVIEW_ANNOUNCEMENT_AT")
	if err != nil {
		return Config{}, err
	}
	announcementAt, err := optionalTimeFromEnv("OPREC_ANNOUNCEMENT_AT")
	if err != nil {
		return Config{}, err
	}

	allowedBatches, err := int16CSVFromEnv("OPREC_ALLOWED_BATCHES", []int16{2024, 2025, 2026})
	if err != nil {
		return Config{}, err
	}

	portfolioMaxBytes, err := int64FromEnv("OPREC_PORTFOLIO_MAX_BYTES", 10*1024*1024)
	if err != nil {
		return Config{}, err
	}

	bodyLimit, err := int64FromEnv("REQUEST_BODY_LIMIT_BYTES", 12*1024*1024)
	if err != nil {
		return Config{}, err
	}

	maxOpenConnections, err := intFromEnv("DB_MAX_OPEN_CONNS", 20)
	if err != nil {
		return Config{}, err
	}

	maxIdleConnections, err := intFromEnv("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return Config{}, err
	}

	driveTimeout, err := durationFromEnv("GDRIVE_TIMEOUT", time.Minute)
	if err != nil {
		return Config{}, err
	}

	storage := StorageConfig{
		Driver:             envOrDefault("PORTFOLIO_STORAGE_DRIVER", StorageDriverLocal),
		PortfolioDirectory: envOrDefault("PORTFOLIO_STORAGE_DIR", "./data/portfolios"),
		Drive: DriveConfig{
			ClientID:     strings.TrimSpace(os.Getenv("GDRIVE_CLIENT_ID")),
			ClientSecret: strings.TrimSpace(os.Getenv("GDRIVE_CLIENT_SECRET")),
			RefreshToken: strings.TrimSpace(os.Getenv("GDRIVE_REFRESH_TOKEN")),
			FolderID:     strings.TrimSpace(os.Getenv("GDRIVE_FOLDER_ID")),
			Timeout:      driveTimeout,
		},
	}
	if err := storage.validate(); err != nil {
		return Config{}, err
	}

	environment := envOrDefault("ENVIRONMENT", "development")
	jwtSecret := envOrDefault("JWT_SECRET", "development-only-change-me")
	if environment == "production" && (jwtSecret == "development-only-change-me" || len(jwtSecret) < 32) {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters in production")
	}
	if bodyLimit <= portfolioMaxBytes {
		return Config{}, fmt.Errorf("REQUEST_BODY_LIMIT_BYTES must be greater than OPREC_PORTFOLIO_MAX_BYTES")
	}
	adminEmail := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	if adminEmail != "" && adminEmail != "admin@petrolida.com" {
		return Config{}, fmt.Errorf("ADMIN_EMAIL must be admin@petrolida.com")
	}
	oprec := OprecConfig{
		RegistrationStartAt:     registrationStartAt,
		RegistrationEndAt:       registrationEndAt,
		AnnouncementAt:          announcementAt,
		InterviewAnnouncementAt: interviewAnnouncementAt,
		StaffGroupURL:           strings.TrimSpace(os.Getenv("OPREC_STAFF_GROUP_URL")),
		InterviewScheduleURL:    strings.TrimSpace(os.Getenv("OPREC_INTERVIEW_SCHEDULE_URL")),
		AllowedBatches:          allowedBatches,
		PortfolioMaxBytes:       portfolioMaxBytes,
		PortfolioRequired: splitCSV(envOrDefault(
			"OPREC_PORTFOLIO_REQUIRED_SUBDIVISIONS",
			"Creative Design,Experimental Design,Media Production (Editing),Media Production (Field),Web Development - UI/UX",
		)),
	}
	if err := oprec.validate(); err != nil {
		return Config{}, err
	}
	publicURL, err := publicURLFromEnv("APP_PUBLIC_URL")
	if err != nil {
		return Config{}, err
	}

	petroRun, err := loadPetroRunConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		App: AppConfig{
			Name:        envOrDefault("APP_NAME", "petrolida-2027-api"),
			Environment: environment,
			LogLevel:    envOrDefault("LOG_LEVEL", "info"),
			PublicURL:   publicURL,
		},
		HTTP: HTTPConfig{
			Host:             envOrDefault("APP_HOST", "127.0.0.1"),
			Port:             envOrDefault("APP_PORT", "8998"),
			AllowedOrigins:   splitCSV(envOrDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
			ReadTimeout:      readTimeout,
			WriteTimeout:     writeTimeout,
			IdleTimeout:      idleTimeout,
			ShutdownTimeout:  shutdownTimeout,
			RequestBodyLimit: bodyLimit,
		},
		Database: DatabaseConfig{
			Host:            envOrDefault("DB_HOST", "127.0.0.1"),
			Port:            envOrDefault("DB_PORT", "54327"),
			User:            envOrDefault("DB_USER", "postgres"),
			Password:        envOrDefault("DB_PASS", "postgres"),
			Name:            envOrDefault("DB_NAME", "petrolida_2027"),
			SSLMode:         envOrDefault("DB_SSL_MODE", "disable"),
			Timezone:        envOrDefault("DB_TIMEZONE", "Asia/Jakarta"),
			MaxOpenConns:    maxOpenConnections,
			MaxIdleConns:    maxIdleConnections,
			ConnMaxLifetime: connectionLifetime,
		},
		Auth: AuthConfig{
			JWTSecret:      jwtSecret,
			SessionTTL:     sessionTTL,
			CookieName:     envOrDefault("AUTH_COOKIE_NAME", "petrolida_admin_session"),
			CookieSecure:   environment == "production",
			AdminName:      strings.TrimSpace(os.Getenv("ADMIN_NAME")),
			AdminEmail:     adminEmail,
			AdminPassword:  os.Getenv("ADMIN_PASSWORD"),
			AdminInstansi:  envOrDefault("ADMIN_INSTITUTION", "PETROLIDA 2027"),
			AdminNoTelepon: strings.TrimSpace(os.Getenv("ADMIN_PHONE")),
		},
		Oprec:    oprec,
		Storage:  storage,
		PetroRun: petroRun,
	}, nil
}

func publicURLFromEnv(key string) (string, error) {
	value := strings.TrimSuffix(strings.TrimSpace(os.Getenv(key)), "/")
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s must be an absolute HTTP(S) origin without path", key)
	}
	return value, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func int64FromEnv(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func optionalTimeFromEnv(key string) (time.Time, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func int16CSVFromEnv(key string, fallback []int16) ([]int16, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parts := splitCSV(value)
	result := make([]int16, 0, len(parts))
	for _, part := range parts {
		parsed, err := strconv.ParseInt(part, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", key, err)
		}
		result = append(result, int16(parsed))
	}
	return result, nil
}
