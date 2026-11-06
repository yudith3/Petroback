package main

import (
	"os"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/cmd"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/app"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
	"github.com/joho/godotenv"
)

func initEnv() {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			logger.Panicf("Failed to load .env file: %v", err)
		}
	}
}

func main() {
	initEnv()

	if os.Getenv("SENTRY_ENABLED") == "true" {
		if err := logger.InitSentry(); err != nil {
			logger.Warnf("[SENTRY] Failed to initialize: %v", err)
		}

		defer func() {
			if ok := logger.FlushSentry(2 * time.Second); !ok {
				logger.Warnf("[SENTRY] Timeout flushing events")
			}
		}()
	}

	if err := cmd.Commands(); err != nil {
		logger.Errorf("[CMD] Error: %v", err)
		os.Exit(1)
	}

	logger.Infof("[MAIN] Starting REST API...")

	restAPI, err := app.NewRest()
	if err != nil {
		logger.Errorf("[MAIN] Failed to initialize REST API: %v", err)
		os.Exit(1)
	}
	defer func() {
		if restAPI != nil && restAPI.DB != nil {
			if closeErr := restAPI.DB.Close(); closeErr != nil {
				logger.Errorf("[MAIN] Failed to close database: %v", closeErr)
			}
		}
	}()

	if err := restAPI.Start(); err != nil {
		logger.Errorf("[ERROR] Failed to start server: %v", err)
		os.Exit(1)
	}
}
