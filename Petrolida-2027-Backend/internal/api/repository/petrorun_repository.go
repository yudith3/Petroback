package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GORM menamai field PayableTotalIDR menjadi payable_total_id_r.
// Ganti ke payable_total_idr kalau tag column di entity sudah diperbaiki.
const petroRunPayableColumn = "payable_total_id_r"

// Order dengan status ini masih bisa menerima pembayaran/review,
// jadi nominalnya tidak boleh dipakai order lain.
var petroRunActiveStatuses = []entity.PetroRunOrderStatus{
	entity.PetroRunWaitingPayment,
	entity.PetroRunPendingReview,
}

type PetroRunRepository interface {
	CountEarlyBirdReservations(ctx context.Context, now time.Time) (int, error)
	CreatePricedOrder(ctx context.Context, now time.Time, rules domain.PetroRunSaleRules, surcharge int64, order *entity.PetroRunOrder) error
	ListAdminOrders(ctx context.Context, query domain.PetroRunAdminQuery) ([]PetroRunAdminRecord, int64, error)
	GetAdminOrder(ctx context.Context, orderNumber string) (PetroRunAdminRecord, error)
	SummarizeAdminOrders(ctx context.Context, query domain.PetroRunAdminQuery) (domain.PetroRunSummary, error)
	GetByIdempotencyIdKey(ctx context.Context, key uuid.UUID) (entity.PetroRunOrder, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (entity.PetroRunOrder, error)
	CreateOrder(ctx context.Context, time time.Time, baseTotal int64, order *entity.PetroRunOrder) error
	ExpireOverdue(ctx context.Context, time time.Time) (int64, error)
	ApproveOrder(ctx context.Context, orderID uuid.UUID, actorID *uuid.UUID, newTicket func(p entity.PetroRunParticipant, ticketNumber string) (entity.PetroRunTicket, error)) error
	GetOrderWithTickets(ctx context.Context, orderNumber string) (entity.PetroRunOrder, error)
	GetParticipantByTicketNumber(ctx context.Context, ticketNumber string) (entity.PetroRunParticipant, error)
	GetParticipantByTicketTokenHash(ctx context.Context, tokenHash string) (entity.PetroRunParticipant, error)
	SubmitPaymentProof(ctx context.Context, orderNumber string, proof *entity.PetroRunPaymentProof, now time.Time, autoApprove bool, newTicket func(entity.PetroRunParticipant, string) (entity.PetroRunTicket, error)) (PetroRunProofRecord, error)
	ListPaymentProofs(ctx context.Context, status string, offset, limit int) ([]PetroRunProofRecord, int64, error)
	GetPaymentProof(ctx context.Context, proofID uuid.UUID) (PetroRunProofRecord, error)
	ReviewPaymentProof(ctx context.Context, proofID uuid.UUID, approve bool, reviewerID uuid.UUID, note string, now time.Time, newTicket func(entity.PetroRunParticipant, string) (entity.PetroRunTicket, error)) (PetroRunProofRecord, error)
}

type PetroRunProofRecord struct {
	Proof entity.PetroRunPaymentProof
	Order entity.PetroRunOrder
}

type petroRunRepository struct {
	db *gorm.DB
}

func NewpetroRun(db *gorm.DB) PetroRunRepository {
	return &petroRunRepository{db: db}
}

func (r *petroRunRepository) findOrder(ctx context.Context, query string, arg any) (entity.PetroRunOrder, error) {
	var order entity.PetroRunOrder
	err := r.db.WithContext(ctx).Preload("Participants").Where(query, arg).First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return order, domain.ErrPetroRunOrderNotFound
	}
	return order, err
}

func (r *petroRunRepository) GetByIdempotencyIdKey(ctx context.Context, key uuid.UUID) (entity.PetroRunOrder, error) {
	return r.findOrder(ctx, "idempotency_key = ?", key)
}

func pickPetroRunCode(baseTotal int64, used []int64) (int, bool) {
	taken := make(map[int64]bool, len(used))
	for _, total := range used {
		taken[total] = true
	}

	free := make([]int, 0, 999)
	for code := 1; code <= 999; code++ {
		if !taken[baseTotal+domain.PetroRunUniqueCode(baseTotal, code)] {
			free = append(free, code)
		}
	}

	if len(free) == 0 {
		return 0, false
	}

	return free[rand.Intn(len(free))], true
}

func petroRunOrderNumber(sequence int64) string {
	return fmt.Sprintf("PR27-%06d", sequence)
}

func petroRunTicketNumber(sequence int64) string {
	return fmt.Sprintf("PR27-TKT-%03d", sequence)
}

func (r *petroRunRepository) CreateOrder(ctx context.Context, time time.Time, baseTotal int64, order *entity.PetroRunOrder) error {
	return r.createOrder(ctx, time, order, func(*gorm.DB) (int64, error) { return baseTotal, nil })
}

func (r *petroRunRepository) createOrder(ctx context.Context, now time.Time, order *entity.PetroRunOrder, allocatePrice func(*gorm.DB) (int64, error)) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := lockPetroRunAllocation(tx)
		if err != nil {
			return err
		}

		err = expireOverdueUpdate(tx, now).Error
		if err != nil {
			return err
		}
		baseTotal, err := allocatePrice(tx)
		if err != nil {
			return err
		}

		var used []int64
		err = tx.Model(&entity.PetroRunOrder{}).
			Where("status IN ?", petroRunActiveStatuses).
			Where(petroRunPayableColumn+" BETWEEN ? AND ?", baseTotal, baseTotal+999).
			Pluck(petroRunPayableColumn, &used).Error
		if err != nil {
			return err
		}

		code, ok := pickPetroRunCode(baseTotal, used)
		if !ok {
			return domain.ErrPetroRunExhausted
		}

		order.UniqueCode = code
		order.UniqueAdjustmentIDR = domain.PetroRunUniqueCode(baseTotal, code)
		order.PayableTotalIDR = baseTotal + order.UniqueAdjustmentIDR
		var orderSequence int64
		err = tx.Raw("SELECT nextval('petro_run_order_number_seq')").Scan(&orderSequence).Error
		if err != nil {
			return fmt.Errorf("allocate PetroRun order number: %w", err)
		}
		order.OrderNumber = petroRunOrderNumber(orderSequence)

		// Participants ikut ter-insert lewat association.
		err = tx.Create(order).Error
		if err != nil {
			return err
		}

		return tx.Create(&entity.PetroRunAuditLog{
			OrderID:  order.ID,
			Action:   "order_created",
			ToStatus: string(order.Status),
		}).Error
	})
	return mapPetroRunError(err)
}

func mapPetroRunError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return domain.ErrPetroRunConflict
	}
	return err
}

func (r *petroRunRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (entity.PetroRunOrder, error) {
	return r.findOrder(ctx, "order_number = ?", orderNumber)
}

func (r *petroRunRepository) ExpireOverdue(ctx context.Context, time time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPetroRunAllocation(tx); err != nil {
			return err
		}
		expired := expireOverdueUpdate(tx, time)
		count = expired.RowsAffected
		return expired.Error
	})
	return count, err
}

func expireOverdueUpdate(db *gorm.DB, now time.Time) *gorm.DB {
	return db.Model(&entity.PetroRunOrder{}).Where("status = ? AND expires_at <= ?", entity.PetroRunWaitingPayment, now).Updates(map[string]any{"status": entity.PetroRunExpired, "early_bird_reserved": false})
}

// ApproveOrder mengubah status ke approved dan menerbitkan tiket per peserta
// dalam satu transaksi. Kalau satu tiket gagal, status ikut di-rollback,
// jadi tidak ada order approved tanpa tiket.
func (r *petroRunRepository) ApproveOrder(ctx context.Context, orderID uuid.UUID, actorID *uuid.UUID, newTicket func(p entity.PetroRunParticipant, ticketNumber string) (entity.PetroRunTicket, error)) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPetroRunAllocation(tx); err != nil {
			return err
		}
		// FOR UPDATE: approve yang datang bersamaan menunggu di sini sampai
		// transaksi pertama selesai, lalu melihat status yang sudah approved.
		var order entity.PetroRunOrder
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Participants").
			Preload("Participants.Ticket").
			First(&order, "id = ?", orderID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrPetroRunOrderNotFound
		}
		if err != nil {
			return err
		}

		switch order.Status {
		case entity.PetroRunApproved:
			// Approve ulang: status tidak diubah, peserta yang belum punya
			// tiket tetap dilengkapi di bawah.
		case entity.PetroRunWaitingPayment, entity.PetroRunPendingReview:
			if err := reservePetroRunEarlyBirdTx(tx, &order, time.Now().UTC()); err != nil {
				return err
			}
			fromStatus := order.Status
			err = tx.Model(&order).Update("status", entity.PetroRunApproved).Error
			if err != nil {
				return err
			}
			err = tx.Create(&entity.PetroRunAuditLog{
				OrderID:    order.ID,
				ActorID:    actorID,
				Action:     "order_approved",
				FromStatus: string(fromStatus),
				ToStatus:   string(entity.PetroRunApproved),
			}).Error
			if err != nil {
				return err
			}
		default:
			return domain.ErrPetroRunNotApprovable
		}

		if err := createPetroRunTicketsTx(tx, order, newTicket); err != nil {
			return err
		}
		return nil
	})
	return mapPetroRunError(err)
}

func createPetroRunTicketsTx(tx *gorm.DB, order entity.PetroRunOrder, newTicket func(entity.PetroRunParticipant, string) (entity.PetroRunTicket, error)) error {
	for _, participant := range order.Participants {
		if participant.Ticket != nil {
			continue
		}
		var ticketSequence int64
		if err := tx.Raw("SELECT nextval('petro_run_ticket_number_seq')").Scan(&ticketSequence).Error; err != nil {
			return fmt.Errorf("allocate PetroRun ticket number: %w", err)
		}
		ticket, err := newTicket(participant, petroRunTicketNumber(ticketSequence))
		if err != nil {
			return err
		}
		err = tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "participant_id"}},
			DoNothing: true,
		}).Create(&ticket).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *petroRunRepository) SubmitPaymentProof(
	ctx context.Context,
	orderNumber string,
	proof *entity.PetroRunPaymentProof,
	now time.Time,
	autoApprove bool,
	newTicket func(entity.PetroRunParticipant, string) (entity.PetroRunTicket, error),
) (PetroRunProofRecord, error) {
	var record PetroRunProofRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPetroRunAllocation(tx); err != nil {
			return err
		}
		var order entity.PetroRunOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Participants.Ticket").
			Where("order_number = ?", orderNumber).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrPetroRunOrderNotFound
			}
			return err
		}

		var existing entity.PetroRunPaymentProof
		err := tx.Where("order_id = ? AND file_sha256 = ?", order.ID, proof.FileSHA256).
			Order("attempt DESC").First(&existing).Error
		if err == nil {
			record = PetroRunProofRecord{Proof: existing, Order: order}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if order.Status == entity.PetroRunApproved {
			return domain.ErrPetroRunOrderPaid
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "petrorun_proof:"+proof.FileSHA256).Error; err != nil {
			return err
		}
		var duplicateCount int64
		if err := tx.Model(&entity.PetroRunPaymentProof{}).
			Where("file_sha256 = ? AND order_id <> ?", proof.FileSHA256, order.ID).
			Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			proof.OCRMismatchReasons = appendProofReason(proof.OCRMismatchReasons, "duplicate_proof_other_order")
			autoApprove = false
		}

		var maxAttempt int
		if err := tx.Model(&entity.PetroRunPaymentProof{}).
			Where("order_id = ?", order.ID).Select("COALESCE(MAX(attempt), 0)").Scan(&maxAttempt).Error; err != nil {
			return err
		}
		proof.OrderID = order.ID
		proof.Attempt = maxAttempt + 1
		proof.LateSubmission = !now.Before(order.ExpiresAt)
		proof.CreatedAt = now
		proof.UpdatedAt = now

		if proof.OCRClassification == "not_receipt" {
			proof.Status = "rejected_non_receipt"
			proof.RejectionReason = "client OCR classified upload as not a bank receipt"
			if err := tx.Create(proof).Error; err != nil {
				return err
			}
			if order.Status == entity.PetroRunWaitingPayment && proof.LateSubmission {
				if err := tx.Model(&order).Updates(map[string]any{"status": entity.PetroRunExpired, "early_bird_reserved": false}).Error; err != nil {
					return err
				}
				order.Status = entity.PetroRunExpired
			}
			if err := tx.Create(&entity.PetroRunAuditLog{
				OrderID: order.ID, Action: "payment_proof_rejected_non_receipt",
				FromStatus: string(order.Status), ToStatus: string(order.Status),
				Reason: fmt.Sprintf("proof_id=%s; attempt=%d; source=browser_client", proof.ID, proof.Attempt),
			}).Error; err != nil {
				return err
			}
			record = PetroRunProofRecord{Proof: *proof, Order: order}
			return nil
		}
		// A late replacement is always admitted to review. Reacquire released
		// early-bird slots when available, without changing the issued invoice.
		if err := reservePetroRunEarlyBirdTx(tx, &order, now); err != nil {
			if !errors.Is(err, domain.ErrPetroRunEarlyBirdExhausted) {
				return err
			}
			proof.OCRMismatchReasons = appendProofReason(proof.OCRMismatchReasons, "early_bird_reservation_released")
			autoApprove = false
			if err := tx.Model(&order).Update("early_bird_reserved", false).Error; err != nil {
				return err
			}
			order.EarlyBirdReserved = false
		}

		if autoApprove && !proof.LateSubmission && order.Status == entity.PetroRunWaitingPayment {
			proof.Status = "approved"
			reviewedAt := now
			proof.ReviewedAt = &reviewedAt
			if err := tx.Create(proof).Error; err != nil {
				return err
			}
			fromStatus := order.Status
			if err := tx.Model(&order).Update("status", entity.PetroRunApproved).Error; err != nil {
				return err
			}
			order.Status = entity.PetroRunApproved
			if err := tx.Create(&entity.PetroRunAuditLog{
				OrderID: order.ID, Action: "payment_auto_approved",
				FromStatus: string(fromStatus), ToStatus: string(order.Status),
				Reason: fmt.Sprintf("proof_id=%s; attempt=%d; source=browser_client_ocr", proof.ID, proof.Attempt),
			}).Error; err != nil {
				return err
			}
			if err := createPetroRunTicketsTx(tx, order, newTicket); err != nil {
				return err
			}
		} else {
			proof.Status = "pending_review"
			if err := tx.Create(proof).Error; err != nil {
				return err
			}
			fromStatus := order.Status
			if err := tx.Model(&order).Update("status", entity.PetroRunPendingReview).Error; err != nil {
				return err
			}
			order.Status = entity.PetroRunPendingReview
			if err := tx.Create(&entity.PetroRunAuditLog{
				OrderID: order.ID, Action: "payment_proof_submitted",
				FromStatus: string(fromStatus), ToStatus: string(order.Status),
				Reason: fmt.Sprintf("proof_id=%s; attempt=%d; source=browser_client_ocr; reasons=%s", proof.ID, proof.Attempt, proof.OCRMismatchReasons),
			}).Error; err != nil {
				return err
			}
		}
		record = PetroRunProofRecord{Proof: *proof, Order: order}
		return nil
	})
	return record, mapPetroRunError(err)
}

func appendProofReason(reasons, reason string) string {
	for _, existing := range strings.Split(reasons, "\n") {
		if existing == reason {
			return reasons
		}
	}
	if reasons == "" {
		return reason
	}
	return reasons + "\n" + reason
}

func (r *petroRunRepository) ListPaymentProofs(ctx context.Context, status string, offset, limit int) ([]PetroRunProofRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.PetroRunPaymentProof{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var proofs []entity.PetroRunPaymentProof
	if err := query.Preload("Order").Order("created_at DESC, attempt DESC").Offset(offset).Limit(limit).Find(&proofs).Error; err != nil {
		return nil, 0, err
	}
	records := make([]PetroRunProofRecord, len(proofs))
	for i, proof := range proofs {
		records[i] = PetroRunProofRecord{Proof: proof, Order: proof.Order}
	}
	return records, total, nil
}

func (r *petroRunRepository) GetPaymentProof(ctx context.Context, proofID uuid.UUID) (PetroRunProofRecord, error) {
	var proof entity.PetroRunPaymentProof
	if err := r.db.WithContext(ctx).Preload("Order").Where("id = ?", proofID).First(&proof).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PetroRunProofRecord{}, domain.ErrPetroRunProofNotFound
		}
		return PetroRunProofRecord{}, err
	}
	return PetroRunProofRecord{Proof: proof, Order: proof.Order}, nil
}

func (r *petroRunRepository) ReviewPaymentProof(
	ctx context.Context,
	proofID uuid.UUID,
	approve bool,
	reviewerID uuid.UUID,
	note string,
	now time.Time,
	newTicket func(entity.PetroRunParticipant, string) (entity.PetroRunTicket, error),
) (PetroRunProofRecord, error) {
	var record PetroRunProofRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPetroRunAllocation(tx); err != nil {
			return err
		}
		var proof entity.PetroRunPaymentProof
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", proofID).First(&proof).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrPetroRunProofNotFound
			}
			return err
		}
		var order entity.PetroRunOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Participants.Ticket").First(&order, "id = ?", proof.OrderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrPetroRunOrderNotFound
			}
			return err
		}
		if proof.Status == "approved" && approve && order.Status == entity.PetroRunApproved {
			record = PetroRunProofRecord{Proof: proof, Order: order}
			return nil
		}
		if proof.Status == "rejected" && !approve && order.Status == entity.PetroRunRejected {
			record = PetroRunProofRecord{Proof: proof, Order: order}
			return nil
		}
		if proof.Status != "pending_review" {
			return domain.ErrPetroRunProofReviewed
		}
		var latestAttempt int
		if err := tx.Model(&entity.PetroRunPaymentProof{}).Where("order_id = ? AND status <> ?", order.ID, "rejected_non_receipt").Select("COALESCE(MAX(attempt), 0)").Scan(&latestAttempt).Error; err != nil {
			return err
		}
		if proof.Attempt != latestAttempt {
			return domain.ErrPetroRunProofReviewed
		}
		if order.Status == entity.PetroRunApproved || order.Status == entity.PetroRunRejected {
			return domain.ErrPetroRunNotApprovable
		}

		reviewedAt := now
		proof.ReviewerID = &reviewerID
		proof.ReviewedAt = &reviewedAt
		proof.ReviewNote = note
		fromStatus := order.Status
		toStatus := entity.PetroRunRejected
		action := "payment_rejected_manual"
		proof.Status = "rejected"
		if approve {
			if err := reservePetroRunEarlyBirdTx(tx, &order, now); err != nil {
				return err
			}
			toStatus = entity.PetroRunApproved
			action = "payment_approved_manual"
			proof.Status = "approved"
		}
		if err := tx.Model(&proof).Updates(map[string]any{
			"status": proof.Status, "reviewer_id": reviewerID, "reviewed_at": reviewedAt, "review_note": note,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&order).Updates(map[string]any{"status": toStatus, "early_bird_reserved": approve && order.EarlyBirdReserved}).Error; err != nil {
			return err
		}
		order.Status = toStatus
		if err := tx.Create(&entity.PetroRunAuditLog{
			OrderID: order.ID, ActorID: &reviewerID, Action: action,
			FromStatus: string(fromStatus), ToStatus: string(toStatus),
			Reason: fmt.Sprintf("proof_id=%s; source=admin_manual; note=%s", proof.ID, note),
		}).Error; err != nil {
			return err
		}
		if approve {
			if err := createPetroRunTicketsTx(tx, order, newTicket); err != nil {
				return err
			}
		}
		record = PetroRunProofRecord{Proof: proof, Order: order}
		return nil
	})
	return record, mapPetroRunError(err)
}

func (r *petroRunRepository) GetOrderWithTickets(ctx context.Context, orderNumber string) (entity.PetroRunOrder, error) {
	var order entity.PetroRunOrder
	// Diurutkan supaya urutan tiket sama setiap kali link dibuka ulang.
	err := r.db.WithContext(ctx).
		Preload("Participants", func(db *gorm.DB) *gorm.DB { return db.Order("id") }).
		Preload("Participants.Ticket").
		Where("order_number = ?", orderNumber).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return order, domain.ErrPetroRunOrderNotFound
	}
	return order, err
}

func (r *petroRunRepository) GetParticipantByTicketNumber(ctx context.Context, ticketNumber string) (entity.PetroRunParticipant, error) {
	return r.findApprovedTicketParticipant(ctx, "petro_run_tickets.ticket_number = ?", ticketNumber)
}

func (r *petroRunRepository) GetParticipantByTicketTokenHash(ctx context.Context, tokenHash string) (entity.PetroRunParticipant, error) {
	return r.findApprovedTicketParticipant(ctx, "petro_run_tickets.token_hash = ?", tokenHash)
}

// Hanya tiket dari order approved yang dikembalikan, supaya tiket order yang
// belum atau tidak lagi approved tidak bocor lewat cek tiket atau scan QR.
func (r *petroRunRepository) findApprovedTicketParticipant(ctx context.Context, query string, arg any) (entity.PetroRunParticipant, error) {
	var participant entity.PetroRunParticipant
	err := r.db.WithContext(ctx).
		Preload("Ticket").
		Joins("JOIN petro_run_tickets ON petro_run_tickets.participant_id = petro_run_participants.id").
		Joins("JOIN petro_run_orders ON petro_run_orders.id = petro_run_participants.order_id").
		Where("petro_run_orders.status = ?", entity.PetroRunApproved).
		Where(query, arg).
		First(&participant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return participant, domain.ErrPetroRunTicketNotFound
	}
	return participant, err
}
