package domain

import (
	"time"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/google/uuid"
)

var (
	ErrShortLinkNotFound  = myerror.New(myerror.TypeNotFound, "SHORT_LINK_NOT_FOUND", "short link not found")
	ErrShortLinkSlugTaken = myerror.New(myerror.TypeConflict, "SHORT_LINK_SLUG_TAKEN", "short link alias is already in use")
)

type CreateShortLinkInput struct {
	Slug           string `json:"slug"`
	DestinationURL string `json:"destination_url"`
}

type ShortLinkResponse struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	DestinationURL string    `json:"destination_url"`
	ClickCount     int64     `json:"click_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
