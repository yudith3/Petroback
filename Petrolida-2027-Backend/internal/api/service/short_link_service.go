package service

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/google/uuid"
)

var shortLinkSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,46}[a-z0-9])$`)

type ShortLinkService interface {
	Create(input domain.CreateShortLinkInput, actorID uuid.UUID) (domain.ShortLinkResponse, error)
	List() ([]domain.ShortLinkResponse, error)
	Delete(id uuid.UUID) error
	Resolve(slug string) (string, error)
}

type shortLinkService struct {
	repository repository.ShortLinkRepository
}

func NewShortLink(repository repository.ShortLinkRepository) ShortLinkService {
	return &shortLinkService{repository: repository}
}

func (s *shortLinkService) Create(
	input domain.CreateShortLinkInput,
	actorID uuid.UUID,
) (domain.ShortLinkResponse, error) {
	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	if !shortLinkSlugPattern.MatchString(slug) {
		return domain.ShortLinkResponse{}, shortLinkFieldError(
			"slug",
			"Alias",
			"must contain 3 to 48 lowercase letters, numbers, or hyphens and cannot start or end with a hyphen",
		)
	}

	destination, err := normalizeShortLinkDestination(input.DestinationURL)
	if err != nil {
		return domain.ShortLinkResponse{}, shortLinkFieldError(
			"destination_url",
			"Destination URL",
			"must be a complete HTTP or HTTPS URL without embedded credentials",
		)
	}

	link := entity.ShortLink{
		Slug:           slug,
		DestinationURL: destination,
		CreatedBy:      actorID,
	}
	if err := s.repository.Create(&link); err != nil {
		return domain.ShortLinkResponse{}, err
	}
	return mapShortLink(link), nil
}

func (s *shortLinkService) List() ([]domain.ShortLinkResponse, error) {
	links, err := s.repository.List()
	if err != nil {
		return nil, err
	}
	items := make([]domain.ShortLinkResponse, 0, len(links))
	for _, link := range links {
		items = append(items, mapShortLink(link))
	}
	return items, nil
}

func (s *shortLinkService) Delete(id uuid.UUID) error {
	return s.repository.Delete(id)
}

func (s *shortLinkService) Resolve(slug string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(slug))
	if !shortLinkSlugPattern.MatchString(normalized) {
		return "", domain.ErrShortLinkNotFound
	}
	link, err := s.repository.Resolve(normalized)
	if err != nil {
		return "", err
	}
	return link.DestinationURL, nil
}

func normalizeShortLinkDestination(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 2048 {
		return "", domain.ErrInvalidInput
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", domain.ErrInvalidInput
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", domain.ErrInvalidInput
	}
	return parsed.String(), nil
}

func mapShortLink(link entity.ShortLink) domain.ShortLinkResponse {
	return domain.ShortLinkResponse{
		ID:             link.ID,
		Slug:           link.Slug,
		DestinationURL: link.DestinationURL,
		ClickCount:     link.ClickCount,
		CreatedAt:      link.Timestamp.CreatedAt,
		UpdatedAt:      link.Timestamp.UpdatedAt,
	}
}

func shortLinkFieldError(field, label, message string) error {
	return myerror.NewValidation([]myerror.FieldError{{
		Field: field, Label: label, Message: message,
	}})
}
