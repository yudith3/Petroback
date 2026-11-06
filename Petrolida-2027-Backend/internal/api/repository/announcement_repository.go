package repository

import (
	"context"
	"errors"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"gorm.io/gorm"
)

type AnnouncementRepository interface {
	GetByNRP(context.Context, string) (entity.OprecStaff, error)
	GetInterviewResult(context.Context, string) (entity.InterviewResult, error)
}
type announcementRepository struct{ db *gorm.DB }

func NewAnnouncement(db *gorm.DB) AnnouncementRepository { return &announcementRepository{db} }
func (r *announcementRepository) GetByNRP(ctx context.Context, nrp string) (entity.OprecStaff, error) {
	var registrant entity.OprecStaff
	err := r.db.WithContext(ctx).Select("id", "nama", "nrp", "status", "accepted_divisi_id", "current_subdivision_id").
		Preload("AcceptedDivisi").Preload("CurrentSubdivision").Where("nrp = ?", nrp).First(&registrant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return registrant, domain.ErrRegistrantNotFound
	}
	return registrant, err
}

func (r *announcementRepository) GetInterviewResult(ctx context.Context, nrp string) (entity.InterviewResult, error) {
	var result entity.InterviewResult
	err := r.db.WithContext(ctx).Preload("Subdivision").Where("nrp = ?", nrp).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, domain.ErrRegistrantNotFound
	}
	return result, err
}
