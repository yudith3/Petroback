package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/google/uuid"
)

type fakeShortLinkRepository struct {
	createFn  func(*entity.ShortLink) error
	listFn    func() ([]entity.ShortLink, error)
	deleteFn  func(uuid.UUID) error
	resolveFn func(string) (entity.ShortLink, error)
}

func (r fakeShortLinkRepository) Create(link *entity.ShortLink) error {
	if r.createFn != nil {
		return r.createFn(link)
	}
	return nil
}

func (r fakeShortLinkRepository) List() ([]entity.ShortLink, error) {
	if r.listFn != nil {
		return r.listFn()
	}
	return nil, nil
}

func (r fakeShortLinkRepository) Delete(id uuid.UUID) error {
	if r.deleteFn != nil {
		return r.deleteFn(id)
	}
	return nil
}

func (r fakeShortLinkRepository) Resolve(slug string) (entity.ShortLink, error) {
	if r.resolveFn != nil {
		return r.resolveFn(slug)
	}
	return entity.ShortLink{}, domain.ErrShortLinkNotFound
}

func TestShortLinkCreateNormalizesInput(t *testing.T) {
	actorID := uuid.New()
	linkID := uuid.New()
	createdAt := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	service := NewShortLink(fakeShortLinkRepository{
		createFn: func(link *entity.ShortLink) error {
			if link.Slug != "petro-run" {
				t.Fatalf("slug = %q, want petro-run", link.Slug)
			}
			if link.DestinationURL != "https://example.com/events?from=dashboard" {
				t.Fatalf("destination = %q", link.DestinationURL)
			}
			if link.CreatedBy != actorID {
				t.Fatalf("created by = %s, want %s", link.CreatedBy, actorID)
			}
			link.ID = linkID
			link.Timestamp.CreatedAt = createdAt
			link.Timestamp.UpdatedAt = createdAt
			return nil
		},
	})

	got, err := service.Create(domain.CreateShortLinkInput{
		Slug:           "  Petro-Run  ",
		DestinationURL: "  https://example.com/events?from=dashboard  ",
	}, actorID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID != linkID || got.Slug != "petro-run" || !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("Create() = %+v", got)
	}
}

func TestShortLinkCreateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input domain.CreateShortLinkInput
		field string
	}{
		{name: "Short alias", input: domain.CreateShortLinkInput{Slug: "ab", DestinationURL: "https://example.com"}, field: "slug"},
		{name: "Leading hyphen", input: domain.CreateShortLinkInput{Slug: "-event", DestinationURL: "https://example.com"}, field: "slug"},
		{name: "Invalid character", input: domain.CreateShortLinkInput{Slug: "petro_event", DestinationURL: "https://example.com"}, field: "slug"},
		{name: "Missing scheme", input: domain.CreateShortLinkInput{Slug: "event", DestinationURL: "example.com"}, field: "destination_url"},
		{name: "Unsafe scheme", input: domain.CreateShortLinkInput{Slug: "event", DestinationURL: "javascript:alert(1)"}, field: "destination_url"},
		{name: "Embedded credentials", input: domain.CreateShortLinkInput{Slug: "event", DestinationURL: "https://user:password@example.com"}, field: "destination_url"},
		{name: "Excessive URL length", input: domain.CreateShortLinkInput{Slug: "event", DestinationURL: "https://example.com/" + strings.Repeat("a", 2048)}, field: "destination_url"},
	}

	service := NewShortLink(fakeShortLinkRepository{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(test.input, uuid.New())
			var validation myerror.Error
			if !errors.As(err, &validation) || len(validation.Fields) != 1 {
				t.Fatalf("Create() error = %#v, want one validation field", err)
			}
			if validation.Fields[0].Field != test.field {
				t.Fatalf("validation field = %q, want %q", validation.Fields[0].Field, test.field)
			}
		})
	}
}

func TestShortLinkResolveNormalizesSlug(t *testing.T) {
	service := NewShortLink(fakeShortLinkRepository{
		resolveFn: func(slug string) (entity.ShortLink, error) {
			if slug != "petro-run" {
				t.Fatalf("slug = %q, want petro-run", slug)
			}
			return entity.ShortLink{DestinationURL: "https://example.com/events"}, nil
		},
	})

	got, err := service.Resolve(" Petro-Run ")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "https://example.com/events" {
		t.Fatalf("Resolve() = %q", got)
	}
}

func TestShortLinkResolveRejectsMalformedSlugAsNotFound(t *testing.T) {
	service := NewShortLink(fakeShortLinkRepository{})
	_, err := service.Resolve("../admin")
	var notFound myerror.Error
	if !errors.As(err, &notFound) || notFound.Code != "SHORT_LINK_NOT_FOUND" {
		t.Fatalf("Resolve() error = %v, want ErrShortLinkNotFound", err)
	}
}
