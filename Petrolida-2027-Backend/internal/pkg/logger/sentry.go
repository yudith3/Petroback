package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

func InitSentry() error {
	sentryDSN := os.Getenv("SENTRY_DSN")

	if sentryDSN == "" {
		return nil
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	appVersion := os.Getenv("APP_VERSION")
	if appVersion == "" {
		appVersion = "1.0.0"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              sentryDSN,
		Environment:      environment,
		Release:          appVersion,
		TracesSampleRate: 0.1,
		AttachStacktrace: true,
		MaxBreadcrumbs:   100,
		Debug:            environment == "development",
	})

	if err != nil {
		return fmt.Errorf("failed to init sentry: %w", err)
	}

	return nil
}

func CaptureException(err error, tags map[string]string) {
	hub := sentry.CurrentHub()
	if hub == nil {
		return
	}

	if len(tags) > 0 {
		scope := hub.Scope()
		scope.SetTags(tags)
	}

	hub.CaptureException(err)
}

func CaptureMessage(message string) {
	hub := sentry.CurrentHub()
	if hub == nil {
		return
	}

	scope := hub.Scope()
	scope.SetLevel(sentry.LevelError)
	hub.CaptureMessage(message)
}

func AddBreadcrumb(
	message string,
	category string,
	level sentry.Level,
	data map[string]interface{},
) {
	hub := sentry.CurrentHub()
	if hub == nil {
		return
	}

	hub.AddBreadcrumb(&sentry.Breadcrumb{
		Message:   message,
		Category:  category,
		Level:     level,
		Timestamp: time.Now(),
		Data:      data,
	}, nil)
}

func FlushSentry(timeout time.Duration) bool {
	return sentry.Flush(timeout)
}
