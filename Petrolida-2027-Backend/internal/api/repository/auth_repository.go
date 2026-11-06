package repository

import (
	"errors"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthRepository interface {
	FindByEmail(email string) (entity.User, error)
	FindByID(id uuid.UUID) (entity.User, error)
	UpsertSuperAdmin(user entity.User) error
}

type authRepository struct {
	db *gorm.DB
}

func NewAuth(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) FindByEmail(email string) (entity.User, error) {
	var user entity.User
	err := r.db.Preload("Divisi").Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	return user, err
}

func (r *authRepository) FindByID(id uuid.UUID) (entity.User, error) {
	var user entity.User
	err := r.db.Preload("Divisi").First(&user, "id = ?", id).Error
	return user, err
}

func (r *authRepository) UpsertSuperAdmin(user entity.User) error {
	var existing entity.User
	err := r.db.Where("LOWER(email) = LOWER(?)", user.Email).First(&existing).Error
	if err == nil {
		return r.db.Model(&existing).Updates(map[string]any{
			"name":        user.Name,
			"password":    user.Password,
			"instansi":    user.Instansi,
			"no_telp":     user.NoTelp,
			"role":        entity.RoleSuperAdmin,
			"divisi_id":   nil,
			"is_verified": true,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(&user).Error
}
