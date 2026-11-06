package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OprecStaffRepository interface {
	GetBySubmissionKey(ctx context.Context, key uuid.UUID) (entity.OprecStaff, error)
	ValidateReferences(facultyID, departmentID uuid.UUID, subdivisionIDs []uuid.UUID) (map[uuid.UUID]entity.Subdivisi, error)
	Create(registrant *entity.OprecStaff, choices []entity.OprecStaffChoice, skills *entity.OprecStaffSkill, additionalInfo *entity.OprecStaffAdditionalInformation, documents []entity.OprecStaffDocument) error
	List(params domain.RegistrantListParams, divisionID *uuid.UUID) ([]entity.OprecStaff, int64, error)
	ListForExport(params domain.RegistrantListParams, divisionID *uuid.UUID) ([]entity.OprecStaff, error)
	GetByID(id uuid.UUID, divisionID *uuid.UUID) (entity.OprecStaff, error)
	GetDocument(registrantID, documentID uuid.UUID, divisionID *uuid.UUID) (entity.OprecStaffDocument, error)
	UpdateStatus(params UpdateStatusParams) (entity.OprecStaffStatusAudit, error)
	Counts(divisionID *uuid.UUID) (domain.RegistrantCountResponse, error)
	PrimarySubdivisionCounts(divisionID *uuid.UUID) ([]domain.SubdivisionCountResponse, error)
	ListActivityLogs(params domain.ActivityLogListParams) ([]entity.OprecStaffStatusAudit, int64, error)
}

type oprecStaffRepository struct {
	db *gorm.DB
}

// UpdateStatusParams adalah input UpdateStatus. Actor* dicatat di audit log,
// ScopeDivisionID membatasi akses (nil untuk superadmin).
type UpdateStatusParams struct {
	RegistrantID         uuid.UUID
	ActorID              uuid.UUID
	ActorRole            entity.UserRole
	ActorDivisionID      *uuid.UUID
	ScopeDivisionID      *uuid.UUID
	Status               entity.OprecStaffStatus
	AcceptedDivisionID   *uuid.UUID
	CurrentSubdivisionID *uuid.UUID
	RejectionTarget      string
}

func NewOprecStaff(db *gorm.DB) OprecStaffRepository {
	return &oprecStaffRepository{db: db}
}

func (r *oprecStaffRepository) GetBySubmissionKey(ctx context.Context, key uuid.UUID) (entity.OprecStaff, error) {
	var registrant entity.OprecStaff
	err := r.db.WithContext(ctx).Select("id", "submission_fingerprint").Where("submission_key = ?", key).First(&registrant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return registrant, domain.ErrRegistrantNotFound
	}
	return registrant, err
}

func (r *oprecStaffRepository) ValidateReferences(
	facultyID, departmentID uuid.UUID,
	subdivisionIDs []uuid.UUID,
) (map[uuid.UUID]entity.Subdivisi, error) {
	var department entity.Departemen
	if err := r.db.Where("id = ? AND fakultas_id = ?", departmentID, facultyID).First(&department).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrDepartmentIdNotFound
		}
		return nil, err
	}

	var subdivisions []entity.Subdivisi
	if err := r.db.Preload("Divisi").Where("id IN ? AND is_active = ?", subdivisionIDs, true).Find(&subdivisions).Error; err != nil {
		return nil, err
	}
	if len(subdivisions) != len(subdivisionIDs) {
		return nil, domain.ErrSubdivsionIdNotFound
	}

	result := make(map[uuid.UUID]entity.Subdivisi, len(subdivisions))
	for _, subdivision := range subdivisions {
		if subdivision.Divisi == nil || !subdivision.Divisi.IsActive {
			return nil, domain.ErrSubdivsionIdNotFound
		}
		result[subdivision.ID] = subdivision
	}
	return result, nil
}

func (r *oprecStaffRepository) Create(
	registrant *entity.OprecStaff,
	choices []entity.OprecStaffChoice,
	skills *entity.OprecStaffSkill,
	additionalInfo *entity.OprecStaffAdditionalInformation,
	documents []entity.OprecStaffDocument,
) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := requireMutableStaffSelection(tx); err != nil {
			return err
		}
		if err := tx.Create(registrant).Error; err != nil {
			return err
		}
		for index := range choices {
			choices[index].OprecStaffID = registrant.ID
		}
		if err := tx.Create(&choices).Error; err != nil {
			return err
		}
		skills.OprecStaffID = registrant.ID
		if err := tx.Create(skills).Error; err != nil {
			return err
		}
		additionalInfo.OprecStaffID = registrant.ID
		if err := tx.Create(additionalInfo).Error; err != nil {
			return err
		}
		for index := range documents {
			documents[index].OprecStaffID = registrant.ID
		}
		return tx.Create(&documents).Error
	})
	if err == nil {
		return nil
	}

	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return domain.ErrRegistrationExists
	}
	return err
}

// filteredQuery dipakai bersama oleh List dan ListForExport supaya hasil export
// selalu sama dengan daftar yang dilihat admin di dashboard.
func (r *oprecStaffRepository) filteredQuery(params domain.RegistrantListParams, divisionID *uuid.UUID) *gorm.DB {
	query := withDivisionScope(r.db.Model(&entity.OprecStaff{}), divisionID)
	if params.Search != "" {
		value := "%" + strings.ToLower(params.Search) + "%"
		query = query.Where("LOWER(nama) LIKE ? OR LOWER(nrp) LIKE ? OR LOWER(email) LIKE ?", value, value, value)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.FacultyID != nil {
		query = query.Where("fakultas_id = ?", *params.FacultyID)
	}
	if params.DepartmentID != nil {
		query = query.Where("departemen_id = ?", *params.DepartmentID)
	}
	if params.Batch != nil {
		query = query.Where("angkatan = ?", *params.Batch)
	}
	if params.SubdivisionID != nil || params.DivisionID != nil {
		choiceQuery := r.db.Model(&entity.OprecStaffChoice{}).Select("1").
			Joins("JOIN subdivisi s ON s.id = oprec_staff_choices.subdivisi_id AND s.deleted_at IS NULL").
			Where("oprec_staff_choices.oprec_staff_id = oprec_staff.id")
		if params.SubdivisionID != nil {
			choiceQuery = choiceQuery.Where("oprec_staff_choices.subdivisi_id = ?", *params.SubdivisionID)
		}
		if params.DivisionID != nil {
			choiceQuery = choiceQuery.Where("s.divisi_id = ?", *params.DivisionID)
		}
		if divisionID != nil {
			choiceQuery = choiceQuery.Where("s.divisi_id = ?", *divisionID)
		}
		if params.ChoicePriority != 0 {
			choiceQuery = choiceQuery.Where("oprec_staff_choices.pilihan = ?", params.ChoicePriority)
		}
		query = query.Where("EXISTS (?)", choiceQuery)
	}
	return query
}

// orderClause hanya memakai kolom dari whitelist, jadi aman disisipkan ke SQL.
// id dipakai sebagai tie-breaker supaya urutan stabil antar-request.
func orderClause(params domain.RegistrantListParams) string {
	orderColumns := map[string]string{
		"created_at": "created_at",
		"name":       "nama",
		"nrp":        "nrp",
		"status":     "status",
	}
	orderColumn := orderColumns[params.SortBy]
	if orderColumn == "" {
		orderColumn = "created_at"
	}
	direction := "DESC"
	if strings.EqualFold(params.SortDirection, "asc") {
		direction = "ASC"
	}
	return orderColumn + " " + direction + ", id ASC"
}

func (r *oprecStaffRepository) List(params domain.RegistrantListParams, divisionID *uuid.UUID) ([]entity.OprecStaff, int64, error) {
	query := r.filteredQuery(params, divisionID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var registrants []entity.OprecStaff
	offset := (params.Page - 1) * params.PageSize
	err := preloadRegistrant(query).
		Order(orderClause(params)).
		Offset(offset).
		Limit(params.PageSize).
		Find(&registrants).Error
	return registrants, total, err
}

// ListForExport mengambil seluruh hasil filter tanpa pagination.
func (r *oprecStaffRepository) ListForExport(params domain.RegistrantListParams, divisionID *uuid.UUID) ([]entity.OprecStaff, error) {
	var registrants []entity.OprecStaff
	err := preloadRegistrant(r.filteredQuery(params, divisionID)).
		Preload("Documents").
		Order(orderClause(params)).
		Find(&registrants).Error
	return registrants, err
}

func (r *oprecStaffRepository) GetByID(id uuid.UUID, divisionID *uuid.UUID) (entity.OprecStaff, error) {
	var registrant entity.OprecStaff
	err := preloadRegistrant(withDivisionScope(r.db.Model(&entity.OprecStaff{}), divisionID)).
		Preload("Documents").
		First(&registrant, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.OprecStaff{}, domain.ErrRegistrantNotFound
	}
	return registrant, err
}

func (r *oprecStaffRepository) GetDocument(registrantID, documentID uuid.UUID, divisionID *uuid.UUID) (entity.OprecStaffDocument, error) {
	var accessible int64
	if err := withDivisionScope(r.db.Model(&entity.OprecStaff{}), divisionID).
		Where("oprec_staff.id = ?", registrantID).
		Count(&accessible).Error; err != nil {
		return entity.OprecStaffDocument{}, err
	}
	if accessible == 0 {
		return entity.OprecStaffDocument{}, domain.ErrDocumentNotFound
	}
	var document entity.OprecStaffDocument
	err := r.db.Where("id = ? AND oprec_staff_id = ?", documentID, registrantID).First(&document).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.OprecStaffDocument{}, domain.ErrDocumentNotFound
	}
	return document, err
}

func (r *oprecStaffRepository) UpdateStatus(params UpdateStatusParams) (entity.OprecStaffStatusAudit, error) {
	// Nilai ini bisa berubah di dalam transaksi, misalnya saat rejection target diproses.
	status := params.Status
	acceptedDivisionID := params.AcceptedDivisionID
	currentSubdivisionID := params.CurrentSubdivisionID
	scopeDivisionID := params.ScopeDivisionID
	rejectionTarget := params.RejectionTarget

	var audit entity.OprecStaffStatusAudit
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := requireMutableStaffSelection(tx); err != nil {
			return err
		}
		var registrant entity.OprecStaff
		query := withDivisionScope(tx.Model(&entity.OprecStaff{}), scopeDivisionID)
		if err := query.Clauses(clause.Locking{Strength: "UPDATE"}).First(&registrant, "oprec_staff.id = ?", params.RegistrantID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRegistrantNotFound
			}
			return err
		}
		previousStatus := registrant.Status
		previousPrimaryRejected, previousSecondaryRejected := registrant.PrimaryRejected, registrant.SecondaryRejected
		primaryRejected, secondaryRejected := registrant.PrimaryRejected, registrant.SecondaryRejected
		switch status {
		case entity.OprecStaffStatusPending:
			primaryRejected, secondaryRejected = false, false
		case entity.OprecStaffStatusScreeningPrimary:
			primaryRejected = false
		case entity.OprecStaffStatusScreeningSecondary:
			primaryRejected, secondaryRejected = true, false
		case entity.OprecStaffStatusRejected:
			primaryRejected, secondaryRejected = true, true
		}
		if rejectionTarget != "" {
			var choices []entity.OprecStaffChoice
			if err := tx.Preload("Subdivisi.Divisi").Where("oprec_staff_id = ?", registrant.ID).Find(&choices).Error; err != nil {
				return err
			}
			var rejectionErr error
			status, currentSubdivisionID, primaryRejected, secondaryRejected, rejectionErr = resolveRejection(registrant, choices, scopeDivisionID, rejectionTarget)
			if rejectionErr != nil {
				return rejectionErr
			}
			acceptedDivisionID = nil
		}
		if acceptedDivisionID != nil {
			if err := validatePlacementSubdivision(tx, *acceptedDivisionID, scopeDivisionID); err != nil {
				return err
			}
		}
		if currentSubdivisionID != nil && rejectionTarget == "" {
			if err := validatePlacementSubdivision(tx, *currentSubdivisionID, scopeDivisionID); err != nil {
				return err
			}
		}
		// Acceptance/interview is an explicit correction for the selected choice;
		// retain the other division's rejection history.
		placement := acceptedDivisionID
		if placement == nil && status == entity.OprecStaffStatusInterview {
			placement = currentSubdivisionID
		}
		if placement != nil && (primaryRejected || secondaryRejected) {
			var choices []entity.OprecStaffChoice
			if err := tx.Where("oprec_staff_id = ? AND subdivisi_id = ?", registrant.ID, *placement).Find(&choices).Error; err != nil {
				return err
			}
			for _, choice := range choices {
				if choice.Pilihan == 1 {
					primaryRejected = false
				}
				if choice.Pilihan == 2 {
					secondaryRejected = false
				}
			}
		}
		previousDivisionID := registrant.AcceptedDivisiID
		previousCurrentSubdivisionID := registrant.CurrentSubdivisionID
		if err := tx.Model(&registrant).Updates(map[string]any{
			"status":                 status,
			"accepted_divisi_id":     acceptedDivisionID,
			"current_subdivision_id": currentSubdivisionID,
			"primary_rejected":       primaryRejected,
			"secondary_rejected":     secondaryRejected,
		}).Error; err != nil {
			return err
		}
		audit = entity.OprecStaffStatusAudit{
			OprecStaffID: registrant.ID, ActorID: params.ActorID,
			ActorRole: params.ActorRole, ActorDivisionID: params.ActorDivisionID,
			PreviousStatus: previousStatus, NewStatus: status,
			RejectionTarget:         rejectionTarget,
			PreviousPrimaryRejected: previousPrimaryRejected, NewPrimaryRejected: primaryRejected,
			PreviousSecondaryRejected: previousSecondaryRejected, NewSecondaryRejected: secondaryRejected,
			PreviousAcceptedSubdivisionID: previousDivisionID,
			NewAcceptedSubdivisionID:      acceptedDivisionID,
			PreviousCurrentSubdivisionID:  previousCurrentSubdivisionID,
			NewCurrentSubdivisionID:       currentSubdivisionID,
		}
		return tx.Create(&audit).Error
	})
	return audit, err
}

// SHARE lets unrelated admin edits run concurrently while excluding the
// finalizer's UPDATE lock. Holding it until commit closes the check/write race.
func requireMutableStaffSelection(tx *gorm.DB) error {
	var selection entity.StaffSelection
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&selection, "id = ?", entity.StaffSelectionID).Error; err != nil {
		return err
	}
	if selection.FinalizedAt != nil {
		return domain.ErrSelectionFinalized
	}
	return nil
}

func (r *oprecStaffRepository) Counts(divisionID *uuid.UUID) (domain.RegistrantCountResponse, error) {
	var result domain.RegistrantCountResponse
	if err := withDivisionScope(r.db.Model(&entity.OprecStaff{}), divisionID).Count(&result.Register).Error; err != nil {
		return result, err
	}
	rows, err := withDivisionScope(r.db.Model(&entity.OprecStaff{}), divisionID).
		Select("status, COUNT(*) AS count").
		Group("status").Rows()
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return result, err
		}
		switch status {
		case string(entity.OprecStaffStatusPending):
			result.Pending = count
		case string(entity.OprecStaffStatusScreeningPrimary):
			result.ScreeningPrimary = count
		case string(entity.OprecStaffStatusScreeningSecondary):
			result.ScreeningSecondary = count
		case string(entity.OprecStaffStatusInterview):
			result.Interviewed = count
		case string(entity.OprecStaffStatusAccepted):
			result.Accepted = count
		case string(entity.OprecStaffStatusRejected):
			result.Rejected = count
		}
	}
	return result, rows.Err()
}

func (r *oprecStaffRepository) PrimarySubdivisionCounts(divisionID *uuid.UUID) ([]domain.SubdivisionCountResponse, error) {
	var result []domain.SubdivisionCountResponse
	query := r.db.Table("subdivisi AS s").
		Select("s.id AS subdivision_id, s.nama AS subdivision, COUNT(c.id) AS count").
		Joins("LEFT JOIN oprec_staff_choices c ON c.subdivisi_id = s.id AND c.pilihan = ? AND c.deleted_at IS NULL", 1).
		Where("s.deleted_at IS NULL AND s.is_active = ?", true)
	if divisionID != nil {
		query = query.Where("s.divisi_id = ?", *divisionID)
	}
	err := query.
		Group("s.id, s.nama").
		Order("count DESC, s.nama ASC").
		Scan(&result).Error
	return result, err
}

func (r *oprecStaffRepository) ListActivityLogs(
	params domain.ActivityLogListParams,
) ([]entity.OprecStaffStatusAudit, int64, error) {
	query := r.db.Model(&entity.OprecStaffStatusAudit{})
	if params.ActorID != nil {
		query = query.Where("actor_id = ?", *params.ActorID)
	}
	if params.DivisionID != nil {
		query = query.Where("actor_division_id = ?", *params.DivisionID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []entity.OprecStaffStatusAudit
	offset := (params.Page - 1) * params.PageSize
	err := query.
		Preload("Actor").
		Preload("ActorDivision").
		Preload("OprecStaff").
		Preload("PreviousDivision").
		Preload("NewDivision").
		Preload("PreviousAcceptedSubdivision").
		Preload("NewAcceptedSubdivision").
		Preload("PreviousCurrentSubdivision").
		Preload("NewCurrentSubdivision").
		Order("created_at DESC").
		Offset(offset).
		Limit(params.PageSize).
		Find(&logs).Error
	return logs, total, err
}

func withDivisionScope(query *gorm.DB, divisionID *uuid.UUID) *gorm.DB {
	if divisionID == nil {
		return query
	}
	// Legacy interview records have no assignment. Keep them accessible through
	// their original choices until an admin assigns an interview subdivision.
	// Once assigned, only that subdivision's division can access the record.
	return query.Where(`
		(
			(oprec_staff.status = 'accepted' AND EXISTS (
				SELECT 1 FROM subdivisi accepted_subdivision
				WHERE accepted_subdivision.id = oprec_staff.accepted_divisi_id
				  AND accepted_subdivision.deleted_at IS NULL
				  AND accepted_subdivision.divisi_id = ?
			))
			OR (oprec_staff.status = 'interview' AND EXISTS (
				SELECT 1 FROM subdivisi current_subdivision
				WHERE current_subdivision.id = oprec_staff.current_subdivision_id
				  AND current_subdivision.deleted_at IS NULL
				  AND current_subdivision.divisi_id = ?
			))
			OR ((
				oprec_staff.status NOT IN ('accepted', 'interview')
				OR (oprec_staff.status = 'interview' AND oprec_staff.current_subdivision_id IS NULL)
			) AND EXISTS (
			SELECT 1
			FROM oprec_staff_choices c
			JOIN subdivisi s ON s.id = c.subdivisi_id AND s.deleted_at IS NULL
			WHERE c.oprec_staff_id = oprec_staff.id
			  AND c.deleted_at IS NULL
			  AND s.divisi_id = ?
			  AND (oprec_staff.status NOT IN ('screening_primary', 'screening_secondary')
			       OR (oprec_staff.status = 'screening_primary' AND c.pilihan = 1)
			       OR (oprec_staff.status = 'screening_secondary' AND c.pilihan = 2))
			))
		)
	`, *divisionID, *divisionID, *divisionID)
}

func orderedChoicePreload(query *gorm.DB) *gorm.DB {
	return query.Order("pilihan ASC")
}

// validatePlacementSubdivision memastikan subdivisi tujuan aktif, divisinya aktif,
// dan berada dalam scope divisi admin (kalau admin divisi).
func validatePlacementSubdivision(tx *gorm.DB, id uuid.UUID, scopeDivisionID *uuid.UUID) error {
	var subdivision entity.Subdivisi
	if err := tx.Preload("Divisi").Where("id = ? AND is_active = ?", id, true).First(&subdivision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrSubdivsionIdNotFound
		}
		return err
	}
	if subdivision.Divisi == nil || !subdivision.Divisi.IsActive {
		return domain.ErrSubdivsionIdNotFound
	}
	if scopeDivisionID != nil && subdivision.DivisiID != *scopeDivisionID {
		return domain.ErrForbidden
	}
	return nil
}

// preloadRegistrant memuat relasi yang dibutuhkan response pendaftar.
// Documents tidak termasuk karena hanya dipakai di detail dan export.
func preloadRegistrant(query *gorm.DB) *gorm.DB {
	return query.
		Preload("Fakultas").
		Preload("Departemen").
		Preload("AcceptedDivisi").
		Preload("CurrentSubdivision").
		Preload("Choices", orderedChoicePreload).
		Preload("Choices.Subdivisi").
		Preload("Choices.Subdivisi.Divisi").
		Preload("Skills").
		Preload("AdditionalInfo")
}
