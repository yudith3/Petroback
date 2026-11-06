package repository

import (
	"context"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"gorm.io/gorm"
)

// Take this lock before row/proof locks everywhere that changes reservations.
// Order creation, expiry, upload and approval then share the same quota boundary.
func lockPetroRunAllocation(tx *gorm.DB) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext('petrorun_order_alloc'))").Error
}

func countPetroRunEarlyBird(tx *gorm.DB, now time.Time, exclude *entity.PetroRunOrder) (int, error) {
	query := tx.Model(&entity.PetroRunOrder{}).
		Where("early_bird_reserved = ?", true).
		Where("(status IN ? OR (status = ? AND expires_at > ?))", []entity.PetroRunOrderStatus{entity.PetroRunApproved, entity.PetroRunPendingReview}, entity.PetroRunWaitingPayment, now)
	if exclude != nil {
		query = query.Where("id <> ?", exclude.ID)
	}
	var count int
	err := query.Select("COALESCE(SUM(early_bird_quantity), 0)").Scan(&count).Error
	return count, err
}

func (r *petroRunRepository) CountEarlyBirdReservations(ctx context.Context, now time.Time) (int, error) {
	return countPetroRunEarlyBird(r.db.WithContext(ctx), now, nil)
}

func (r *petroRunRepository) CreatePricedOrder(ctx context.Context, now time.Time, rules domain.PetroRunSaleRules, surcharge int64, order *entity.PetroRunOrder) error {
	return r.createOrder(ctx, now, order, func(tx *gorm.DB) (int64, error) {
		if rules.RegistrationStatus(now) != "open" {
			return 0, domain.ErrPetroRunRegistrationClosed
		}
		remaining := 0
		if now.Before(rules.LateStartAt) {
			used, err := countPetroRunEarlyBird(tx, now, nil)
			if err != nil {
				return 0, err
			}
			remaining = max(0, rules.EarlyBirdQuota-used)
		}
		order.BaseSubtotalIDR = 0
		order.EarlyBirdQuantity = 0
		order.EarlyBirdQuota = rules.EarlyBirdQuota
		for i := range order.Participants {
			p := &order.Participants[i]
			p.BasePriceIDR, p.PriceTier = rules.NormalPriceIDR, "normal"
			if !now.Before(rules.LateStartAt) {
				p.BasePriceIDR, p.PriceTier = rules.LatePriceIDR, "late"
			} else if remaining > 0 {
				p.BasePriceIDR, p.PriceTier = rules.EarlyBirdPriceIDR, "early_bird"
				remaining--
				order.EarlyBirdQuantity++
			}
			order.BaseSubtotalIDR += p.BasePriceIDR
		}
		order.EarlyBirdReserved = order.EarlyBirdQuantity > 0
		order.ParticipantSubtotalIDR = order.BaseSubtotalIDR + surcharge
		return order.ParticipantSubtotalIDR + order.AdminFeeIDR, nil
	})
}

func reservePetroRunEarlyBirdTx(tx *gorm.DB, order *entity.PetroRunOrder, now time.Time) error {
	if order.EarlyBirdQuantity == 0 {
		return nil
	}
	if order.EarlyBirdReserved && (order.Status == entity.PetroRunApproved || order.Status == entity.PetroRunPendingReview || (order.Status == entity.PetroRunWaitingPayment && now.Before(order.ExpiresAt))) {
		return nil
	}
	used, err := countPetroRunEarlyBird(tx, now, order)
	if err != nil {
		return err
	}
	if used+order.EarlyBirdQuantity > order.EarlyBirdQuota {
		return domain.ErrPetroRunEarlyBirdExhausted
	}
	if err := tx.Model(order).Update("early_bird_reserved", true).Error; err != nil {
		return err
	}
	order.EarlyBirdReserved = true
	return nil
}
