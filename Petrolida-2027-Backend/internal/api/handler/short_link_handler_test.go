package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeShortLinkService struct {
	resolveFn func(string) (string, error)
}

func (s fakeShortLinkService) Create(
	domain.CreateShortLinkInput,
	uuid.UUID,
) (domain.ShortLinkResponse, error) {
	return domain.ShortLinkResponse{}, nil
}

func (s fakeShortLinkService) List() ([]domain.ShortLinkResponse, error) {
	return nil, nil
}

func (s fakeShortLinkService) Delete(uuid.UUID) error {
	return nil
}

func (s fakeShortLinkService) Resolve(slug string) (string, error) {
	return s.resolveFn(slug)
}

func TestShortLinkRedirectUsesTemporaryRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewShortLink(fakeShortLinkService{
		resolveFn: func(slug string) (string, error) {
			if slug != "petro-run" {
				t.Fatalf("slug = %q, want petro-run", slug)
			}
			return "https://example.com/events", nil
		},
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/go/petro-run", nil)
	context.Params = gin.Params{{Key: "slug", Value: "petro-run"}}

	handler.Redirect(context)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTemporaryRedirect)
	}
	if location := recorder.Header().Get("Location"); location != "https://example.com/events" {
		t.Fatalf("Location = %q", location)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cacheControl)
	}
}
