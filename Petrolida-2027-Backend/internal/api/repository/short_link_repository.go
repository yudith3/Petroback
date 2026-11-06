package repository

import (
	"errors"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type ShortLinkRepository interface {
	Create(link *entity.ShortLink) error
	List() ([]entity.ShortLink, error)
	Delete(id uuid.UUID) error
	Resolve(slug string) (entity.ShortLink, error)
}

type shortLinkRepository struct {
	db *gorm.DB
}

func NewShortLink(db *gorm.DB) ShortLinkRepository {
	return &shortLinkRepository{db: db}
}

func (r *shortLinkRepository) Create(link *entity.ShortLink) error {
	err := r.db.Create(link).Error
	if err == nil {
		return nil
	}

	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return domain.ErrShortLinkSlugTaken
	}
	return err
}

func (r *shortLinkRepository) List() ([]entity.ShortLink, error) {
	links := make([]entity.ShortLink, 0)
	err := r.db.Order("created_at DESC").Find(&links).Error
	return links, err
}

func (r *shortLinkRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&entity.ShortLink{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrShortLinkNotFound
	}
	return nil
}

func (r *shortLinkRepository) Resolve(slug string) (entity.ShortLink, error) {
	var link entity.ShortLink
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("slug = ?", slug).First(&link).Error; err != nil {
			return err
		}
		return tx.Model(&entity.ShortLink{}).
			Where("id = ?", link.ID).
			UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.ShortLink{}, domain.ErrShortLinkNotFound
	}
	if err != nil {
		return entity.ShortLink{}, err
	}
	return link, nil
}
