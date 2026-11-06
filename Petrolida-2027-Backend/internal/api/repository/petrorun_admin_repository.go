package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PetroRunAdminRecord struct {
	Order  entity.PetroRunOrder
	Proofs []entity.PetroRunPaymentProof
	Audit  []entity.PetroRunAuditLog
}

func petroRunAdminFilter(db *gorm.DB, q domain.PetroRunAdminQuery) *gorm.DB {
	db = db.Model(&entity.PetroRunOrder{})
	if q.Status != "" {
		db = db.Where("petro_run_orders.status = ?", q.Status)
	}
	if q.Search != "" {
		term := "%" + strings.TrimSpace(q.Search) + "%"
		db = db.Where(`(order_number ILIKE ? OR registrant_name ILIKE ? OR registrant_email ILIKE ? OR registrant_phone ILIKE ? OR EXISTS (SELECT 1 FROM petro_run_participants p LEFT JOIN petro_run_tickets t ON t.participant_id = p.id AND t.deleted_at IS NULL WHERE p.order_id = petro_run_orders.id AND (p.name ILIKE ? OR t.ticket_number ILIKE ?)))`, term, term, term, term, term, term)
	}
	if q.CreatedFrom != nil {
		db = db.Where("petro_run_orders.created_at >= ?", q.CreatedFrom)
	}
	if q.CreatedTo != nil {
		db = db.Where("petro_run_orders.created_at < ?", q.CreatedTo)
	}
	if q.TransactionFrom != nil || q.TransactionTo != nil {
		proofs := db.Session(&gorm.Session{NewDB: true}).Model(&entity.PetroRunPaymentProof{}).Select("1").Where("petro_run_payment_proofs.order_id = petro_run_orders.id")
		if q.TransactionFrom != nil {
			proofs = proofs.Where("ocr_transaction_at >= ?", q.TransactionFrom)
		}
		if q.TransactionTo != nil {
			proofs = proofs.Where("ocr_transaction_at < ?", q.TransactionTo)
		}
		db = db.Where("EXISTS (?)", proofs)
	}
	return db
}

func (r *petroRunRepository) ListAdminOrders(ctx context.Context, q domain.PetroRunAdminQuery) ([]PetroRunAdminRecord, int64, error) {
	db := petroRunAdminFilter(r.db.WithContext(ctx), q)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	columns := map[string]string{"created_at": "created_at", "expires_at": "expires_at", "payable_total": petroRunPayableColumn, "status": "status"}
	column, ok := columns[q.Sort]
	if !ok {
		column = "created_at"
	}
	direction := "DESC"
	if q.Direction == "asc" {
		direction = "ASC"
	}
	var orders []entity.PetroRunOrder
	if err := db.Order("petro_run_orders." + column + " " + direction).Order("petro_run_orders.id DESC").Preload("Participants").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]uuid.UUID, 0, len(orders))
	for _, order := range orders {
		ids = append(ids, order.ID)
	}
	proofs := []entity.PetroRunPaymentProof{}
	if len(ids) > 0 {
		// One latest proof per order, without loading the raw OCR for every historical attempt.
		if err := r.db.WithContext(ctx).Select("DISTINCT ON (order_id) *").Where("order_id IN ?", ids).Order("order_id, attempt DESC").Find(&proofs).Error; err != nil {
			return nil, 0, err
		}
	}
	byOrder := make(map[uuid.UUID]entity.PetroRunPaymentProof, len(proofs))
	for _, p := range proofs {
		byOrder[p.OrderID] = p
	}
	result := make([]PetroRunAdminRecord, 0, len(orders))
	for _, order := range orders {
		record := PetroRunAdminRecord{Order: order}
		if proof, ok := byOrder[order.ID]; ok {
			record.Proofs = []entity.PetroRunPaymentProof{proof}
		}
		result = append(result, record)
	}
	return result, total, nil
}

func (r *petroRunRepository) GetAdminOrder(ctx context.Context, orderNumber string) (PetroRunAdminRecord, error) {
	var record PetroRunAdminRecord
	err := r.db.WithContext(ctx).Preload("Participants", func(db *gorm.DB) *gorm.DB { return db.Order("id") }).Preload("Participants.Ticket").Where("order_number = ?", orderNumber).First(&record.Order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return record, domain.ErrPetroRunOrderNotFound
	}
	if err != nil {
		return record, err
	}
	if err = r.db.WithContext(ctx).Where("order_id = ?", record.Order.ID).Order("attempt DESC").Find(&record.Proofs).Error; err != nil {
		return record, err
	}
	err = r.db.WithContext(ctx).Where("order_id = ?", record.Order.ID).Order("created_at DESC, id DESC").Find(&record.Audit).Error
	return record, err
}

func (r *petroRunRepository) SummarizeAdminOrders(ctx context.Context, q domain.PetroRunAdminQuery) (domain.PetroRunSummary, error) {
	var result domain.PetroRunSummary
	err := petroRunAdminFilter(r.db.WithContext(ctx), q).Select(`COUNT(*) AS total_orders,
COALESCE(SUM((SELECT COUNT(*) FROM petro_run_participants p WHERE p.order_id = petro_run_orders.id)),0) AS total_participants,
COUNT(*) FILTER (WHERE status = 'waiting_payment') AS waiting_payment,
COUNT(*) FILTER (WHERE status = 'pending_review') AS pending_review,
COUNT(*) FILTER (WHERE status = 'approved') AS approved,
COUNT(*) FILTER (WHERE status = 'rejected') AS rejected,
COUNT(*) FILTER (WHERE status = 'expired') AS expired,
COALESCE(SUM(CASE WHEN status = 'approved' THEN ` + petroRunPayableColumn + ` ELSE 0 END),0) AS approved_total`).Scan(&result).Error
	return result, err
}
