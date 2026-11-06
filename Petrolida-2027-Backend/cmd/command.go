package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/db"
	"github.com/Petrolida/Petrolida-2027-Backend/db/migrations"
	seeds "github.com/Petrolida/Petrolida-2027-Backend/db/seeder"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
	mylog "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func Commands() error {
	if _, err := os.Stat(".env"); err == nil {
		godotenv.Load()
	}

	hasCommand := hasCommandFlag()
	if !hasCommand {
		return nil
	}

	// Dijalankan sebelum config.Load karena perintah inilah yang menghasilkan
	// GDRIVE_REFRESH_TOKEN, sehingga saat dipanggil konfigurasinya memang belum
	// lengkap dan validasi storage pasti menolak.
	if hasArg("--gdrive-token") {
		if err := runDriveToken(); err != nil {
			return err
		}
		os.Exit(0)
		return nil
	}

	// if hasArg("--management-report-sql") {
	// 	printManagementReportSQL()
	// 	os.Exit(0)
	// 	return nil
	// }

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// log := logger.New(cfg.App.LogLevel)
	// slog.SetDefault(log)

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	database, err := db.Open(startupContext, cfg.Database)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Errorf("close database: %v", closeErr)
		}
	}()

	if err := executeCommands(database.ORM); err != nil {
		return err
	}

	os.Exit(0)
	return nil
}

func hasCommandFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--migrate" || arg == "--seeder" || arg == "--watch" || arg == "--help" || arg == "--gdrive-token" {
			return true
		}
	}
	return false
}

func hasArg(name string) bool {
	for _, arg := range os.Args[1:] {
		if arg == name {
			return true
		}
	}
	return false
}

func executeCommands(database *gorm.DB) error {
	migrate := false
	seeder := false
	help := false
	watch := false

	for _, arg := range os.Args[1:] {
		if arg == "--migrate" {
			migrate = true
		}
		if arg == "--seeder" {
			seeder = true
		}
		if arg == "--help" {
			help = true
		}
		if arg == "--watch" {
			watch = true
		}
	}

	if migrate {
		mylog.Infof("[MIGRATE] Starting migration...")
		if err := migrations.Migrate(database); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		mylog.Infof("[MIGRATE] ✅ Migration completed successfully")
	}

	if seeder {
		mylog.Infof("[SEEDER] Starting seeding...")
		if err := seeds.RunSeeder(database); err != nil {
			return fmt.Errorf("seeding failed: %w", err)
		}
		mylog.Infof("[SEEDER] ✅ Seeding completed successfully")
	}
	if help {
		printHelp()
	}

	if watch {
		mylog.Infof("[WATCH] Starting file watcher...")
		if err := runWatch(); err != nil {
			return fmt.Errorf("watching failed: %w", err)
		}
		mylog.Infof("[WATCH] ✅ Watch completed")
		return nil
	}

	return nil
}

func runWatch() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", "air -c .air.windows.toml")
	case "linux", "darwin":
		cmd = exec.Command("bash", "-c", "air -c .air.linux.toml")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		mylog.Errorf("[WATCH] Error: %s", err)
		return err
	}

	mylog.Infof("[WATCH] Command executed successfully")
	return nil
}

func printHelp() {
	fmt.Println("==================================================")
	fmt.Println("      Petrolida 27 Backend CLI Help Guide")
	fmt.Println("==================================================")
	fmt.Println("Berikut adalah daftar command yang tersedia:")
	fmt.Println("")
	fmt.Println("1. Menjalankan Server & Watch")
	fmt.Println("   - go run main.go           : Menjalankan server (default)")
	fmt.Println("   - go run main.go --watch   : Menjalankan server dengan live-reload (Air)")
	fmt.Println("     -> Make shortcut         : make watch")
	fmt.Println("")
	fmt.Println("2. Database Migrations & Seeding")
	fmt.Println("   - go run main.go --migrate : Menjalankan migrasi database (GORM AutoMigrate)")
	fmt.Println("     -> Make shortcut         : make migrate")
	fmt.Println("   - go run main.go --seed    : Menjalankan semua seeder utama")
	fmt.Println("     -> Make shortcut         : make seed")
	fmt.Println("")
	fmt.Println("3. Penyimpanan Portofolio (Google Drive)")
	fmt.Println("   - go run . --gdrive-token  : Menerbitkan GDRIVE_REFRESH_TOKEN dan folder Drive")
	fmt.Println("     -> Perlu GDRIVE_CLIENT_ID dan GDRIVE_CLIENT_SECRET di .env")
	fmt.Println("     -> Panduan lengkap        : docs/gdrive-setup.md")
	fmt.Println("")
	fmt.Println("4. Lain-lain")
	fmt.Println("   - go run main.go --help              : Menampilkan panduan ini")
	fmt.Println("     -> Shortcut                        : go run main.go -h")
	fmt.Println("==================================================")
}
