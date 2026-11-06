package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/router"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/logger"
)

type fakePinger struct {
	err  error
	ping func(context.Context) error
}

func (p fakePinger) PingContext(ctx context.Context) error {
	if p.ping != nil {
		return p.ping(ctx)
	}
	return p.err
}

func TestHealthCheckReturnsReady(t *testing.T) {
	engine := router.New(testConfig(), fakePinger{}, logger.New("error"))
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	engine.ServeHTTP(responseRecorder, request)

	if got, want := responseRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
	if requestID := responseRecorder.Header().Get("X-Request-ID"); requestID == "" {
		t.Fatal("X-Request-ID header is empty")
	}
}

func TestHealthCheckReturnsUnavailable(t *testing.T) {
	engine := router.New(testConfig(), fakePinger{err: errors.New("offline")}, logger.New("error"))
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	engine.ServeHTTP(responseRecorder, request)

	if got, want := responseRecorder.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func TestHealthCheckAllowsManagedDatabaseConnectionLatency(t *testing.T) {
	var timeout time.Duration
	pinger := fakePinger{ping: func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("database health context has no deadline")
		}
		timeout = time.Until(deadline)
		return nil
	}}
	engine := router.New(testConfig(), pinger, logger.New("error"))
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	engine.ServeHTTP(responseRecorder, request)

	if got, want := responseRecorder.Code, http.StatusOK; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
	if timeout < 7*time.Second {
		t.Fatalf("database health timeout = %s, want at least 7s", timeout)
	}
}

func testConfig() config.Config {
	return config.Config{
		App: config.AppConfig{Name: "test-api", Environment: "test"},
		HTTP: config.HTTPConfig{
			AllowedOrigins:   []string{"http://localhost:3000"},
			WriteTimeout:     15 * time.Second,
			RequestBodyLimit: 1024,
		},
	}
}
