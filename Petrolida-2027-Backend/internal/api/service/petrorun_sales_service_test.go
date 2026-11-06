package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func TestPetroRunRegistrationGatePreservesIdempotentRecoveryAfterClose(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	repo := &petroRunMemoryRepository{orders: map[uuid.UUID]entity.PetroRunOrder{}}
	svc := newPetroRunTestService(repo, now)
	key := uuid.New()
	first, err := svc.CreateOrder(ctx, key, validPetroRunInput())
	if err != nil {
		t.Fatal(err)
	}
	svc.cfg.Sales = &domain.PetroRunSaleRules{RegistrationStartAt: now.Add(-time.Hour), RegistrationEndAt: now}
	if _, err := svc.CreateOrder(ctx, uuid.New(), validPetroRunInput()); !errors.Is(err, domain.ErrPetroRunRegistrationClosed) {
		t.Fatalf("new order after close: %v", err)
	}
	// The recovery probe sends an intentionally invalid form. A missing key must
	// still return validation (422), allowing the UI to discard a stale key.
	if _, err := svc.CreateOrder(ctx, uuid.New(), domain.CreatePetroRunOrderInput{}); err == nil || errors.Is(err, domain.ErrPetroRunRegistrationClosed) {
		t.Fatalf("missing recovery key: %v", err)
	}
	recovered, err := svc.CreateOrder(ctx, key, domain.CreatePetroRunOrderInput{})
	if err != nil || recovered.OrderID != first.OrderID || recovered.PayableTotal != first.PayableTotal || !recovered.ExpiresAt.Equal(first.ExpiresAt) || repo.creates != 1 {
		t.Fatalf("recovery changed after close: %+v %v", recovered, err)
	}
}

func TestPetroRunMixedInvoiceResponseUsesSavedPrices(t *testing.T) {
	svc := newPetroRunTestService(nil, time.Now())
	order := entity.PetroRunOrder{BasePriceIDR: 150_000, BaseSubtotalIDR: 550_000, ParticipantSubtotalIDR: 565_000, AdminFeeIDR: 2_500, PayableTotalIDR: 568_123, UniqueAdjustmentIDR: 623, Participants: []entity.PetroRunParticipant{{PriceTier: "normal", BasePriceIDR: 150_000}, {PriceTier: "early_bird", BasePriceIDR: 125_000}, {PriceTier: "normal", BasePriceIDR: 150_000}, {PriceTier: "early_bird", BasePriceIDR: 125_000}}}
	svc.cfg.BasePriceIDR = 999_000
	response := svc.toResponse(order)
	if response.Pricing.BaseSubtotal != 550_000 || response.Pricing.JerseySurcharge != 15_000 || len(response.Pricing.Lines) != 2 || response.Pricing.Lines[0].Tier != "early_bird" || response.Pricing.Lines[0].Quantity != 2 || response.Pricing.Lines[1].SubtotalIDR != 300_000 {
		t.Fatalf("mixed snapshot: %+v", response.Pricing)
	}
}
