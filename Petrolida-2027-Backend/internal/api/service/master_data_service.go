package service

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

type (
	MasterDataService interface {
		GetsDivisions() ([]entity.Divisi, error)
		GetDivisionByID(id uuid.UUID) (entity.Divisi, error)
		GetSubDivisionByID(id uuid.UUID) (entity.Subdivisi, error)
		GetSubDivisions() ([]entity.Subdivisi, error)
		GetDepartments() ([]entity.Departemen, error)
		GetDepartmentByID(id uuid.UUID) (entity.Departemen, error)
		GetFaculties() ([]entity.Fakultas, error)
		GetFacultyByID(id uuid.UUID) (entity.Fakultas, error)
	}

	masterDataService struct {
		masterDataRepository repository.MasterDataRepository
	}
)

func NewMasterData(masterDataRepository repository.MasterDataRepository) MasterDataService {
	return &masterDataService{
		masterDataRepository: masterDataRepository,
	}
}

func (s *masterDataService) GetsDivisions() ([]entity.Divisi, error) {
	return s.masterDataRepository.GetsDivisions()
}

func (s *masterDataService) GetDivisionByID(id uuid.UUID) (entity.Divisi, error) {
	return s.masterDataRepository.GetDivisionByID(id)
}

func (s *masterDataService) GetSubDivisionByID(id uuid.UUID) (entity.Subdivisi, error) {
	return s.masterDataRepository.GetSubDivisionByID(id)
}

func (s *masterDataService) GetSubDivisions() ([]entity.Subdivisi, error) {
	return s.masterDataRepository.GetSubDivisions()
}

func (s *masterDataService) GetDepartments() ([]entity.Departemen, error) {
	return s.masterDataRepository.GetDepartments()
}

func (s *masterDataService) GetDepartmentByID(id uuid.UUID) (entity.Departemen, error) {
	return s.masterDataRepository.GetDepartmentByID(id)
}

func (s *masterDataService) GetFaculties() ([]entity.Fakultas, error) {
	return s.masterDataRepository.GetFaculties()
}

func (s *masterDataService) GetFacultyByID(id uuid.UUID) (entity.Fakultas, error) {
	return s.masterDataRepository.GetFacultyByID(id)
}
