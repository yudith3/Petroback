package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func saleRules(t *testing.T) domain.PetroRunSaleRules {
	t.Helper()
	parse := func(value string) time.Time {
		result, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	return domain.PetroRunSaleRules{RegistrationStartAt: parse("2026-10-24T00:00:00+07:00"), RegistrationEndAt: parse("2026-11-23T00:00:00+07:00"), LateStartAt: parse("2026-11-19T00:00:00+07:00"), RacepackAt: parse("2026-11-25T00:00:00+07:00"), NormalPriceIDR: 150_000, EarlyBirdPriceIDR: 125_000, EarlyBirdQuota: 25, LatePriceIDR: 139_000}
}

func saleOrder(now time.Time, count int) *entity.PetroRunOrder {
	order := newPetroRunTestOrder(now, uuid.New())
	template := order.Participants[1]
	order.Participants = make([]entity.PetroRunParticipant, count)
	for i := range order.Participants {
		order.Participants[i] = template
	}
	return order
}

func saleTicket(p entity.PetroRunParticipant, number string) (entity.PetroRunTicket, error) {
	return entity.PetroRunTicket{ID: uuid.New(), ParticipantID: p.ID, TicketNumber: number, TokenHash: uuid.NewString(), Status: "active"}, nil
}

func TestPetroRunConcurrentSalesReserveExactly25PeopleAndMixGroupPrices(t *testing.T) {
	db := openPetroRunTestDB(t)
	repo, rules, ctx := NewpetroRun(db), saleRules(t), context.Background()
	now := rules.RegistrationStartAt
	const groups = 12
	errs := make(chan error, groups)
	var workers sync.WaitGroup
	for range groups {
		workers.Add(1)
		go func() {
			defer workers.Done()
			errs <- repo.CreatePricedOrder(ctx, now, rules, 5_000, saleOrder(now, 4))
		}()
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var orders []entity.PetroRunOrder
	if err := db.Preload("Participants").Find(&orders).Error; err != nil {
		t.Fatal(err)
	}
	early, normal, mixed := 0, 0, 0
	for _, order := range orders {
		groupEarly, base := 0, int64(0)
		for _, p := range order.Participants {
			base += p.BasePriceIDR
			if p.PriceTier == "early_bird" {
				early++
				groupEarly++
				if p.BasePriceIDR != 125_000 {
					t.Fatal("wrong early price")
				}
			} else {
				normal++
				if p.PriceTier != "normal" || p.BasePriceIDR != 150_000 {
					t.Fatal("wrong normal price")
				}
			}
		}
		if groupEarly > 0 && groupEarly < 4 {
			mixed++
		}
		if base != order.BaseSubtotalIDR || order.ParticipantSubtotalIDR != base+5_000 || order.EarlyBirdQuantity != groupEarly || order.PayableTotalIDR != base+5_000+2_500+order.UniqueAdjustmentIDR || !order.ExpiresAt.Equal(now.Add(24*time.Hour)) {
			t.Fatalf("bad invoice: %+v", order)
		}
	}
	used, err := repo.CountEarlyBirdReservations(ctx, now)
	if err != nil || used != 25 || early != 25 || normal != groups*4-25 || mixed != 1 {
		t.Fatalf("used=%d early=%d normal=%d mixed=%d err=%v", used, early, normal, mixed, err)
	}
}

func TestPetroRunExpiryReleasesSlotsAndLateReviewCannotOversell(t *testing.T) {
	db := openPetroRunTestDB(t)
	repo, rules, ctx := NewpetroRun(db), saleRules(t), context.Background()
	rules.EarlyBirdQuota = 2
	now := rules.RegistrationStartAt
	first := saleOrder(now, 2)
	if err := repo.CreatePricedOrder(ctx, now, rules, 0, first); err != nil {
		t.Fatal(err)
	}
	deadline := first.ExpiresAt
	used, err := repo.CountEarlyBirdReservations(ctx, deadline)
	if err != nil || used != 0 {
		t.Fatalf("expired slots still held: %d %v", used, err)
	}
	second := saleOrder(deadline, 2)
	if err := repo.CreatePricedOrder(ctx, deadline, rules, 0, second); err != nil {
		t.Fatal(err)
	}
	if second.EarlyBirdQuantity != 2 {
		t.Fatal("expired slots not reused")
	}
	proof := entity.PetroRunPaymentProof{FileKey: "private", FileSHA256: "old-early-proof", MIMEType: "image/png", OCRClassification: "unknown"}
	record, err := repo.SubmitPaymentProof(ctx, first.OrderNumber, &proof, deadline, false, saleTicket)
	if err != nil || record.Proof.Status != "pending_review" || record.Order.EarlyBirdReserved || !strings.Contains(record.Proof.OCRMismatchReasons, "early_bird_reservation_released") {
		t.Fatalf("late proof: %+v %v", record, err)
	}
	if _, err := repo.ReviewPaymentProof(ctx, proof.ID, true, uuid.New(), "verified", deadline, saleTicket); !errors.Is(err, domain.ErrPetroRunEarlyBirdExhausted) {
		t.Fatalf("overselling approval: %v", err)
	}
	var count int64
	if err := db.Model(&entity.PetroRunTicket{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("tickets issued on failed approval: %d %v", count, err)
	}
	// A rejection releases reservations immediately. The old invoice can then
	// reacquire capacity and approve at its original price, exactly once.
	newProof := entity.PetroRunPaymentProof{FileKey: "private", FileSHA256: "replacement-early-proof", MIMEType: "image/png", OCRClassification: "unknown"}
	if _, err := repo.SubmitPaymentProof(ctx, second.OrderNumber, &newProof, deadline, false, saleTicket); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReviewPaymentProof(ctx, newProof.ID, false, uuid.New(), "transfer invalid", deadline, saleTicket); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := repo.ReviewPaymentProof(ctx, proof.ID, true, uuid.New(), "verified bank statement", deadline, saleTicket); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := repo.GetByOrderNumber(ctx, first.OrderNumber)
	if err != nil || stored.BaseSubtotalIDR != 250_000 || stored.PayableTotalIDR != first.PayableTotalIDR || !stored.EarlyBirdReserved || stored.Status != entity.PetroRunApproved {
		t.Fatalf("invoice changed: %+v %v", stored, err)
	}
	used, err = repo.CountEarlyBirdReservations(ctx, deadline.Add(48*time.Hour))
	if err != nil || used != 2 {
		t.Fatalf("approved quota released: %d %v", used, err)
	}
}

func TestPetroRunPendingReviewHoldsReservationBeyondDeadline(t *testing.T) {
	db := openPetroRunTestDB(t)
	repo, rules, ctx := NewpetroRun(db), saleRules(t), context.Background()
	rules.EarlyBirdQuota = 2
	now := rules.RegistrationStartAt
	first := saleOrder(now, 2)
	if err := repo.CreatePricedOrder(ctx, now, rules, 0, first); err != nil {
		t.Fatal(err)
	}
	proof := entity.PetroRunPaymentProof{FileKey: "private", FileSHA256: "pending-early-proof", MIMEType: "image/png", OCRClassification: "unknown"}
	if _, err := repo.SubmitPaymentProof(ctx, first.OrderNumber, &proof, now, false, saleTicket); err != nil {
		t.Fatal(err)
	}
	second := saleOrder(first.ExpiresAt, 1)
	if err := repo.CreatePricedOrder(ctx, first.ExpiresAt, rules, 0, second); err != nil {
		t.Fatal(err)
	}
	if second.EarlyBirdQuantity != 0 || second.BaseSubtotalIDR != 150_000 {
		t.Fatalf("pending reservation released: %+v", second)
	}
}

func TestPetroRunSaleBoundariesAndLatePriceOverrideRemainingEarlySlots(t *testing.T) {
	db := openPetroRunTestDB(t)
	repo, rules, ctx := NewpetroRun(db), saleRules(t), context.Background()
	for _, test := range []struct {
		now    time.Time
		price  int64
		tier   string
		closed bool
	}{
		{rules.RegistrationStartAt.Add(-time.Microsecond), 0, "", true},
		{rules.RegistrationStartAt, 125_000, "early_bird", false},
		{rules.LateStartAt.Add(-time.Microsecond), 125_000, "early_bird", false},
		{rules.LateStartAt, 139_000, "late", false},
		{rules.RegistrationEndAt.Add(-time.Microsecond), 139_000, "late", false},
		{rules.RegistrationEndAt, 0, "", true},
	} {
		order := saleOrder(test.now, 1)
		err := repo.CreatePricedOrder(ctx, test.now, rules, 0, order)
		if test.closed {
			if !errors.Is(err, domain.ErrPetroRunRegistrationClosed) {
				t.Fatalf("boundary %s: %v", test.now, err)
			}
			continue
		}
		if err != nil || order.BaseSubtotalIDR != test.price || order.Participants[0].PriceTier != test.tier {
			t.Fatalf("boundary %s: %+v %v", test.now, order, err)
		}
	}
}
