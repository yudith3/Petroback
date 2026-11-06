package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

func New(level string) *slog.Logger {
	logLevel := slog.LevelInfo
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	case "info":
		logLevel = slog.LevelInfo
	default:
		logLevel = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}

// func New(level string) *slog.Logger {
// 	logLevel := slog.LevelInfo
// 	switch strings.ToLower(level) {
// 	case "debug":
// 		logLevel = slog.LevelDebug
// 	case "warn":
// 		logLevel = slog.LevelWarn
// 	case "error":
// 		logLevel = slog.LevelError
// 	}

// 	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
// }

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

func Infof(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logrus.Infof("%s", ColorizeInfo(message))

	AddBreadcrumb(message, "log.info", sentry.LevelInfo, nil)
}

func Warnf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logrus.Warnf("%s", ColorizeWarn(message))

	AddBreadcrumb(message, "log.warn", sentry.LevelWarning, nil)
}

func Errorf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logrus.Errorf("%s", ColorizeError(message))

	AddBreadcrumb(message, "log.error", sentry.LevelError, nil)
}

func Panicf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	logrus.Panicf("%s", ColorizePanic(message))

	// Capture panic
	hub := sentry.CurrentHub()
	if hub != nil {
		scope := hub.Scope()
		scope.SetLevel(sentry.LevelFatal)
		hub.CaptureMessage(message)
	}

	panic(message)
}

func Infoln(v ...any) {
	if len(v) == 1 {
		b, err := json.MarshalIndent(v[0], "", "  ")
		if err == nil {
			logrus.Info("\n" + ColorizeINFOJSON(string(b)))
			AddBreadcrumb(string(b), "log.info.json", sentry.LevelInfo, nil)
			return
		}
	}

	message := strings.TrimSuffix(fmt.Sprintln(v...), "\n")
	logrus.Info(message)
	AddBreadcrumb(message, "log.info", sentry.LevelInfo, nil)
}

func Warnln(v ...any) {
	if len(v) == 1 {
		b, err := json.MarshalIndent(v[0], "", "  ")
		if err == nil {
			logrus.Warn("\n" + ColorizeWarnJSON(string(b)))
			AddBreadcrumb(string(b), "log.warn.json", sentry.LevelWarning, nil)
			return
		}
	}

	message := strings.TrimSuffix(fmt.Sprintln(v...), "\n")
	logrus.Warn(message)
	AddBreadcrumb(message, "log.warn", sentry.LevelWarning, nil)
}

func Errorln(v ...any) {
	if len(v) == 1 {
		b, err := json.MarshalIndent(v[0], "", "  ")
		if err == nil {
			logrus.Error("\n" + ColorizeErrorJSON(string(b)))
			AddBreadcrumb(string(b), "log.error.json", sentry.LevelError, nil)
			return
		}
	}

	message := strings.TrimSuffix(fmt.Sprintln(v...), "\n")
	logrus.Error(message)
	AddBreadcrumb(message, "log.error", sentry.LevelError, nil)
}

func Debug(v ...any) {
	message := fmt.Sprint(v...)
	logrus.Debug(message)
}
