package repository

import (
	"errors"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type (
	MasterDataRepository interface {
		GetsDivisions() ([]entity.Divisi, error)
		GetDivisionByID(id uuid.UUID) (entity.Divisi, error)
		GetSubDivisionByID(id uuid.UUID) (entity.Subdivisi, error)
		GetNameSubDivision(id uuid.UUID) (string, error)
		GetSubDivisions() ([]entity.Subdivisi, error)
		GetDepartments() ([]entity.Departemen, error)
		GetDepartmentByID(uuid.UUID) (entity.Departemen, error)
		GetFaculties() ([]entity.Fakultas, error)
		GetFacultyByID(uuid.UUID) (entity.Fakultas, error)
	}

	masterDataRepository struct {
		db *gorm.DB
	}
)

func NewMasterData(db *gorm.DB) MasterDataRepository {
	return &masterDataRepository{
		db: db,
	}
}

func (r *masterDataRepository) GetsDivisions() ([]entity.Divisi, error) {
	var data []entity.Divisi

	if err := r.db.
		Where("is_active = ?", true).
		Preload("Subdivisi", "is_active = ?", true).
		Find(&data).Error; err != nil {
		return nil, err
	}

	return data, nil
}

func (r *masterDataRepository) GetDivisionByID(id uuid.UUID) (entity.Divisi, error) {
	var data entity.Divisi

	if err := r.db.Where("id = ?", id).Preload("SubDivisions").First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Divisi{}, domain.ErrDivisionIdNotFound
		}
		return entity.Divisi{}, err
	}

	return data, nil
}

func (r *masterDataRepository) GetSubDivisionByID(id uuid.UUID) (entity.Subdivisi, error) {
	var data entity.Subdivisi

	if err := r.db.Where("id = ?", id).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Subdivisi{}, domain.ErrSubdivsionIdNotFound
		}
		return entity.Subdivisi{}, err
	}

	return data, nil
}

func (r *masterDataRepository) GetNameSubDivision(id uuid.UUID) (string, error) {
	var data entity.Subdivisi

	if err := r.db.Select("name").Where("id = ?", id).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", domain.ErrSubdivsionIdNotFound
		}
		return "", err
	}

	return data.Nama, nil
}

func (r *masterDataRepository) GetSubDivisions() ([]entity.Subdivisi, error) {
	var data []entity.Subdivisi

	if err := r.db.Where("is_active = ?", true).Find(&data).Error; err != nil {
		return nil, err
	}

	return data, nil
}

func (r *masterDataRepository) GetDepartments() ([]entity.Departemen, error) {
	var data []entity.Departemen

	if err := r.db.Find(&data).Error; err != nil {
		return nil, err
	}

	return data, nil
}

func (r *masterDataRepository) GetDepartmentByID(id uuid.UUID) (entity.Departemen, error) {
	var data entity.Departemen

	if err := r.db.Where("id = ?", id).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Departemen{}, domain.ErrDepartmentIdNotFound
		}
		return entity.Departemen{}, err
	}

	return data, nil
}

func (r *masterDataRepository) GetFaculties() ([]entity.Fakultas, error) {
	var data []entity.Fakultas

	if err := r.db.Find(&data).Error; err != nil {
		return nil, err
	}

	return data, nil
}

func (r *masterDataRepository) GetFacultyByID(id uuid.UUID) (entity.Fakultas, error) {
	var data entity.Fakultas

	if err := r.db.Where("id = ?", id).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Fakultas{}, domain.ErrFacultyIdNotFound
		}
		return entity.Fakultas{}, err
	}

	return data, nil
}
