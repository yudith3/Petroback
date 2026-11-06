package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/db"
	"github.com/Petrolida/Petrolida-2027-Backend/db/migrations"
	seeder "github.com/Petrolida/Petrolida-2027-Backend/db/seeder"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/handler"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	apiRouter "github.com/Petrolida/Petrolida-2027-Backend/internal/api/router"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
	portfoliostorage "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/storage"
	"github.com/gin-gonic/gin"
)

func newPortfolioStore(cfg config.StorageConfig) (portfoliostorage.Store, error) {
	if cfg.Driver == config.StorageDriverDrive {
		return portfoliostorage.NewDriveStorage(portfoliostorage.DriveCredentials{
			ClientID:     cfg.Drive.ClientID,
			ClientSecret: cfg.Drive.ClientSecret,
			RefreshToken: cfg.Drive.RefreshToken,
			FolderID:     cfg.Drive.FolderID,
			Timeout:      cfg.Drive.Timeout,
		})
	}
	return portfoliostorage.NewPortfolioStorage(cfg.PortfolioDirectory)
}

type RestConfig struct {
	Config config.Config
	DB     *db.Connection
	Engine *gin.Engine
}

func NewRest() (*RestConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

	// Managed database poolers may need more than ten seconds to establish a
	// connection after an idle period. Keep startup bounded, but allow enough
	// time to survive a cold Supabase connection without entering a restart loop.
	startupContext, cancelStartup := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelStartup()

	database, err := db.Open(startupContext, cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := migrations.Migrate(database.ORM); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	if err := seeder.RunSeeder(database.ORM); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("seed master data: %w", err)
	}
	var selection entity.StaffSelection
	if err := database.ORM.WithContext(startupContext).First(&selection, "id = ?", entity.StaffSelectionID).Error; err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("load staff selection lock: %w", err)
	}
	cfg.Oprec.SelectionFinalized = selection.FinalizedAt != nil

	engine := apiRouter.New(cfg, database.SQL, logger.New(cfg.App.LogLevel))

	masterDataRepo := repository.NewMasterData(database.ORM)
	masterDataSvc := service.NewMasterData(masterDataRepo)
	masterDataHandler := handler.NewMasterData(masterDataSvc)
	apiRouter.MasterData(engine, masterDataHandler)

	portfolioStorage, err := newPortfolioStore(cfg.Storage)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	authRepo := repository.NewAuth(database.ORM)
	authSvc := service.NewAuth(authRepo, cfg.Auth)
	if err := authSvc.ProvisionAdmin(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("provision admin: %w", err)
	}
	authHandler := handler.NewAuth(authSvc, cfg.Auth)
	oprecRepo := repository.NewOprecStaff(database.ORM)
	oprecSvc := service.NewOprecStaff(oprecRepo, cfg.Oprec, portfolioStorage, cfg.App.PublicURL)
	oprecHandler := handler.NewOprecStaff(oprecSvc)
	apiRouter.OprecStaff(engine, oprecHandler, authHandler, authSvc, cfg)
	announcementSvc := service.NewAnnouncement(repository.NewAnnouncement(database.ORM), cfg.Oprec)
	apiRouter.Announcements(engine, handler.NewAnnouncement(announcementSvc))

	shortLinkRepo := repository.NewShortLink(database.ORM)
	shortLinkSvc := service.NewShortLink(shortLinkRepo)
	shortLinkHandler := handler.NewShortLink(shortLinkSvc)
	apiRouter.ShortLinks(engine, shortLinkHandler, authSvc, cfg)

	petroRunRepo := repository.NewpetroRun(database.ORM)
	petroRunSvc := service.NewPetroRunService(petroRunRepo, cfg.PetroRun, portfolioStorage)
	apiRouter.PetroRun(engine, handler.NewPetroRun(cfg.PetroRun, petroRunSvc), authSvc, cfg)

	return &RestConfig{
		Config: cfg,
		DB:     database,
		Engine: engine,
	}, nil
}

func (ap *RestConfig) Start() error {
	if ap == nil || ap.Engine == nil {
		return fmt.Errorf("rest server is not initialized")
	}

	host := os.Getenv("APP_HOST")
	if host == "" {
		host = ap.Config.HTTP.Host
	}
	if host == "" {
		host = "127.0.0.1"
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = ap.Config.HTTP.Port
	}
	if port == "" {
		port = "8998"
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", host, port),
		Handler: ap.Engine,
		// ReadTimeout dan WriteTimeout sengaja dibiarkan nol. Unggahan portofolio
		// sampai 10MB dari koneksi seluler gampang melewati batas itu, dan batas
		// waktu di sini akan memutus pendaftaran yang sah di tengah jalan.
		// Yang dibatasi cukup pembacaan header dan koneksi menganggur.
		ReadHeaderTimeout: ap.Config.HTTP.ReadTimeout,
		IdleTimeout:       ap.Config.HTTP.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return err
	case <-signals:
	}

	// Deploy mengganti container, jadi tanpa ini pendaftaran yang sedang
	// mengunggah akan terputus begitu proses lama dimatikan.
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), ap.Config.HTTP.ShutdownTimeout)
	defer cancelShutdown()
	return server.Shutdown(shutdownContext)
}
