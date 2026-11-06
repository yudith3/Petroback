package migrations

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrateRelaxesLegacyChoiceDivisionConstraint(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set; skipping PostgreSQL migration test")
	}

	admin := openMigrationTestDB(t, "")
	schemaName := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schemaName)).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schemaName)).Error
		closeMigrationTestDB(t, admin)
	})

	database := openMigrationTestDB(t, schemaName)
	t.Cleanup(func() { closeMigrationTestDB(t, database) })

	if err := Migrate(database); err != nil {
		t.Fatalf("create current schema: %v", err)
	}
	if !database.Migrator().HasTable("short_links") {
		t.Fatal("migration did not create short_links table")
	}
	if err := database.Exec(`
		ALTER TABLE oprec_staff_choices
		ADD COLUMN divisi_id uuid NOT NULL
	`).Error; err != nil {
		t.Fatalf("add legacy division column: %v", err)
	}

	if err := Migrate(database); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}

	var nullable string
	if err := database.Raw(`
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = ?
		  AND table_name = 'oprec_staff_choices'
		  AND column_name = 'divisi_id'
	`, schemaName).Scan(&nullable).Error; err != nil {
		t.Fatalf("inspect legacy division column: %v", err)
	}
	if nullable != "YES" {
		t.Fatalf("legacy divisi_id is_nullable = %q, want YES", nullable)
	}
}

func TestMigrateEnablesRowLevelSecurity(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set; skipping PostgreSQL migration test")
	}

	admin := openMigrationTestDB(t, "")
	schemaName := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schemaName)).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schemaName)).Error
		closeMigrationTestDB(t, admin)
	})

	database := openMigrationTestDB(t, schemaName)
	t.Cleanup(func() { closeMigrationTestDB(t, database) })

	if err := Migrate(database); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	var unrestricted []string
	if err := database.Raw(`
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = ?
		  AND c.relkind = 'r'
		  AND NOT c.relrowsecurity
		ORDER BY c.relname
	`, schemaName).Scan(&unrestricted).Error; err != nil {
		t.Fatalf("inspect row level security: %v", err)
	}
	if len(unrestricted) != 0 {
		t.Fatalf("tables without row level security: %v", unrestricted)
	}
}

func TestMigrateLegacyDivisionAdminAccounts(t *testing.T) {
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set; skipping PostgreSQL migration test")
	}

	admin := openMigrationTestDB(t, "")
	schemaName := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schemaName)).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schemaName)).Error
		closeMigrationTestDB(t, admin)
	})

	database := openMigrationTestDB(t, schemaName)
	t.Cleanup(func() { closeMigrationTestDB(t, database) })
	if err := Migrate(database); err != nil {
		t.Fatalf("create current schema: %v", err)
	}

	accounts := []struct {
		email        string
		legacyName   string
		divisionName string
	}{
		{email: "fnc@petrolida.com", legacyName: "Finance", divisionName: "Finance"},
		{email: "crtv@petrolida.com", legacyName: "Creative", divisionName: "Creative"},
		{email: "damn@petrolida.com", legacyName: "Data Management", divisionName: "Data Management"},
		{email: "opr@petrolida.com", legacyName: "Operational", divisionName: "Operational"},
		{email: "ncomp@petrolida.com", legacyName: "Non Competition", divisionName: "Non Competition"},
		{email: "comp@petrolida.com", legacyName: "Competition", divisionName: "Competition"},
		{email: "hc@petrolida.com", legacyName: "Human Capital", divisionName: "Human Capital"},
		{email: "pr@petrolida.com", legacyName: "Public Relation", divisionName: "Public Relations"},
	}

	for _, account := range accounts {
		division := entity.Divisi{Nama: account.divisionName, IsActive: true}
		if err := database.Create(&division).Error; err != nil {
			t.Fatalf("create division %q: %v", account.divisionName, err)
		}
		user := entity.User{
			Name: account.legacyName, Email: account.email, Password: "existing-hash",
			Instansi: "Petrolida", NoTelp: "-", Role: entity.UserRole("admin"), IsVerified: true,
		}
		if err := database.Create(&user).Error; err != nil {
			t.Fatalf("create legacy account %q: %v", account.email, err)
		}
	}
	superadmin := entity.User{
		Name: "Admin", Email: "admin@petrolida.com", Password: "superadmin-hash",
		Instansi: "Petrolida", NoTelp: "-", Role: entity.UserRole("admin"), IsVerified: true,
	}
	if err := database.Create(&superadmin).Error; err != nil {
		t.Fatalf("create legacy superadmin: %v", err)
	}

	if err := migrateLegacyAdminRoles(database); err != nil {
		t.Fatalf("migrate legacy admin roles: %v", err)
	}

	for _, account := range accounts {
		var user entity.User
		if err := database.Preload("Divisi").Where("email = ?", account.email).First(&user).Error; err != nil {
			t.Fatalf("load migrated account %q: %v", account.email, err)
		}
		if user.Role != entity.RoleDivisionAdmin {
			t.Errorf("account %q role = %q, want %q", account.email, user.Role, entity.RoleDivisionAdmin)
		}
		if user.Divisi == nil || user.Divisi.Nama != account.divisionName {
			t.Errorf("account %q division = %#v, want %q", account.email, user.Divisi, account.divisionName)
		}
		if user.Password != "existing-hash" {
			t.Errorf("account %q password was changed during migration", account.email)
		}
	}

	var migratedSuperadmin entity.User
	if err := database.Where("email = ?", superadmin.Email).First(&migratedSuperadmin).Error; err != nil {
		t.Fatalf("load migrated superadmin: %v", err)
	}
	if migratedSuperadmin.Role != entity.RoleSuperAdmin || migratedSuperadmin.DivisiID != nil {
		t.Errorf("superadmin role/division = %q/%v", migratedSuperadmin.Role, migratedSuperadmin.DivisiID)
	}
	if migratedSuperadmin.Password != "superadmin-hash" {
		t.Error("superadmin password was changed during migration")
	}
}

func openMigrationTestDB(t *testing.T, searchPath string) *gorm.DB {
	t.Helper()

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	databaseName := os.Getenv("DB_NAME")
	if databaseName == "" {
		databaseName = "postgres"
	}
	sslMode := os.Getenv("DB_SSL_MODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASS")),
		Host:   net.JoinHostPort(os.Getenv("DB_HOST"), port),
		Path:   databaseName,
	}
	query := databaseURL.Query()
	query.Set("sslmode", sslMode)
	if searchPath != "" {
		query.Set("search_path", searchPath)
	}
	databaseURL.RawQuery = query.Encode()

	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return database
}

func closeMigrationTestDB(t *testing.T, database *gorm.DB) {
	t.Helper()
	sqlDatabase, err := database.DB()
	if err != nil {
		t.Errorf("get SQL database: %v", err)
		return
	}
	if err := sqlDatabase.Close(); err != nil {
		t.Errorf("close SQL database: %v", err)
	}
}
