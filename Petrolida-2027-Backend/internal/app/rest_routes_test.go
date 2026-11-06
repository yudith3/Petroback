package app

import (
	"os"
	"testing"
)

func TestNewRestRegistersMasterDataRoutes(t *testing.T) {
	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "true" {
		t.Skip("set RUN_DATABASE_INTEGRATION_TESTS=true with a test PostgreSQL database")
	}
	rest, err := NewRest()
	if err != nil {
		t.Fatalf("NewRest() error = %v", err)
	}
	defer func() {
		if rest != nil && rest.DB != nil {
			_ = rest.DB.Close()
		}
	}()

	found := false
	for _, route := range rest.Engine.Routes() {
		if route.Method == "GET" && route.Path == "/api/v1/master/faculty" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected master faculty route to be registered")
	}
}
