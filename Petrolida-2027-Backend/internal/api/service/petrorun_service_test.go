package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

// petroRunMemoryRepository meniru allocator repository dengan kode tetap.
// Method yang tidak di-override akan panic kalau terpanggil.
type petroRunMemoryRepository struct {
	repository.PetroRunRepository
	orders   map[uuid.UUID]entity.PetroRunOrder
	creates  int
	conflict bool
}

func (r *petroRunMemoryRepository) GetByIdempotencyIdKey(_ context.Context, key uuid.UUID) (entity.PetroRunOrder, error) {
	order, ok := r.orders[key]
	if !ok {
		return order, domain.ErrPetroRunOrderNotFound
	}
	return order, nil
}

func (r *petroRunMemoryRepository) CreateOrder(_ context.Context, _ time.Time, baseTotal int64, order *entity.PetroRunOrder) error {
	r.creates++
	if r.conflict {
		// Simulasi request lain dengan key sama yang menang lebih dulu.
		winner := *order
		winner.ID = uuid.New()
		winner.OrderNumber = "PR27-000001"
		r.orders[*order.IdempotencyKey] = winner
		return domain.ErrPetroRunConflict
	}
	order.ID = uuid.New()
	order.UniqueCode = 167
	order.UniqueAdjustmentIDR = domain.PetroRunUniqueCode(baseTotal, 167)
	order.PayableTotalIDR = baseTotal + order.UniqueAdjustmentIDR
	order.OrderNumber = "PR27-000001"
	r.orders[*order.IdempotencyKey] = *order
	return nil
}

var petroRunTestConfig = config.PetroRunConfig{
	BasePriceIDR:      150_000,
	AdminFeeIDR:       2_500,
	JerseySurcharges:  map[string]int64{"2XL": 5_000, "3XL": 10_000, "4XL": 15_000},
	BankName:          "BNI",
	BankAccountNumber: "1234567890",
	BankAccountHolder: "Petrolida ITS",
	AccessSecret:      "test-secret-at-least-32-characters-long",
}

func newPetroRunTestService(repo repository.PetroRunRepository, now time.Time) *petroRunService {
	return &petroRunService{repository: repo, cfg: petroRunTestConfig, now: func() time.Time { return now }}
}

func validPetroRunInput() domain.CreatePetroRunOrderInput {
	return domain.CreatePetroRunOrderInput{
		FullName:                " Budi Santoso ",
		Email:                   "Budi@Mail.com",
		WhatsappNumber:          "0812-3456-7890",
		RegistrantIsParticipant: true,
		AcceptTerms:             true,
		AcceptPrivacy:           true,
		Participants: []domain.PetroRunParticipantInput{
			{FullName: "Budi Santoso", JerseySize: "xxl", PaceTarget: "6:00/km", MedicalHistory: "tidak ada"},
			{FullName: "Ani Lestari", JerseySize: "M", PaceTarget: "7:00/km", MedicalHistory: "asma ringan"},
		},
	}
}

func TestPetroRunCreateOrderPricesOnServer(t *testing.T) {
	repo := &petroRunMemoryRepository{orders: map[uuid.UUID]entity.PetroRunOrder{}}
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	result, err := newPetroRunTestService(repo, now).CreateOrder(context.Background(), uuid.New(), validPetroRunInput())
	if err != nil {
		t.Fatal(err)
	}

	// 2 x 150.000 + surcharge XXL 5.000 + admin 2.500 = 307.500.
	// Kode 167 butuh adjustment 667 supaya total berakhiran 167: 308.167.
	pricing := result.Pricing
	if pricing.BaseSubtotal != 300_000 || pricing.JerseySurcharge != 5_000 || pricing.AdminFee != 2_500 {
		t.Fatalf("pricing = %+v", pricing)
	}
	if result.PayableTotal != 300_000+5_000+2_500+pricing.UniqueAdjustment || result.PayableTotal%1000 != 167 || result.PayableTotal != 308_167 {
		t.Fatalf("payable = %d, adjustment = %d", result.PayableTotal, pricing.UniqueAdjustment)
	}
	if result.OrderID != "PR27-000001" || result.Status != string(entity.PetroRunWaitingPayment) {
		t.Fatalf("order id = %s, status = %s", result.OrderID, result.Status)
	}
	if !result.ExpiresAt.Equal(result.CreatedAt.Add(24*time.Hour)) || !result.CreatedAt.Equal(now) || result.CreatedAt.Location() != time.UTC {
		t.Fatalf("created = %s, expires = %s", result.CreatedAt, result.ExpiresAt)
	}
	if result.Bank.AccountNumber != "1234567890" || result.AccessToken == "" {
		t.Fatalf("bank = %+v, token = %q", result.Bank, result.AccessToken)
	}

	var stored entity.PetroRunOrder
	for _, order := range repo.orders {
		stored = order
	}
	if stored.RegistrantPhone != "+6281234567890" || stored.RegistrantEmail != "budi@mail.com" || stored.RegistrantName != "Budi Santoso" {
		t.Fatalf("registrant not normalized: %+v", stored)
	}
	if len(stored.Participants) != 2 || stored.Participants[0].JerseySize != "2XL" {
		t.Fatalf("participants = %+v", stored.Participants)
	}
}

func TestPetroRunCreateOrderUsesConfiguredJerseySurcharges(t *testing.T) {
	for size, want := range map[string]int64{"3XL": 10_000, "4XL": 15_000} {
		t.Run(size, func(t *testing.T) {
			repo := &petroRunMemoryRepository{orders: map[uuid.UUID]entity.PetroRunOrder{}}
			input := validPetroRunInput()
			input.Participants[0].JerseySize = size

			result, err := newPetroRunTestService(repo, time.Now()).CreateOrder(context.Background(), uuid.New(), input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Pricing.JerseySurcharge != want {
				t.Fatalf("jersey surcharge = %d, want %d", result.Pricing.JerseySurcharge, want)
			}
		})
	}
}

func TestPetroRunTicketNumberFormatValidation(t *testing.T) {
	for _, test := range []struct {
		number string
		valid  bool
	}{
		{number: "PR27-TKT-001", valid: true},
		{number: "PR27-TKT-1234", valid: true},
		{number: "PR27-TKT-01", valid: false},
		{number: "PR27-RUN-ABCD-EFGH", valid: true},
		{number: "PR27-TKT-00A", valid: false},
		{number: "PR27-RUN-ABC1-EFGH", valid: false},
	} {
		if got := isValidPetroRunTicketNumber(test.number); got != test.valid {
			t.Errorf("isValidPetroRunTicketNumber(%q) = %v, want %v", test.number, got, test.valid)
		}
	}
}

func TestPetroRunCreateOrderIsIdempotent(t *testing.T) {
	repo := &petroRunMemoryRepository{orders: map[uuid.UUID]entity.PetroRunOrder{}}
	svc := newPetroRunTestService(repo, time.Now())
	key := uuid.New()

	first, err := svc.CreateOrder(context.Background(), key, validPetroRunInput())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateOrder(context.Background(), key, validPetroRunInput())
	if err != nil {
		t.Fatal(err)
	}
	if repo.creates != 1 || first.OrderID != second.OrderID || first.AccessToken != second.AccessToken || first.PayableTotal != second.PayableTotal {
		t.Fatalf("retry created a new order: creates=%d first=%+v second=%+v", repo.creates, first, second)
	}
}

func TestPetroRunCreateOrderReturnsWinnerOnConcurrentKey(t *testing.T) {
	repo := &petroRunMemoryRepository{orders: map[uuid.UUID]entity.PetroRunOrder{}, conflict: true}
	key := uuid.New()
	result, err := newPetroRunTestService(repo, time.Now()).CreateOrder(context.Background(), key, validPetroRunInput())
	if err != nil || result.OrderID != repo.orders[key].OrderNumber {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestPetroRunCreateOrderRejectsBeforeStorage(t *testing.T) {
	// Repository nil: kalau request ini sampai ke storage, test akan panic.
	svc := &petroRunService{cfg: petroRunTestConfig, now: time.Now}
	if _, err := svc.CreateOrder(context.Background(), uuid.Nil, validPetroRunInput()); err == nil {
		t.Fatal("empty idempotency key accepted")
	}

	unavailable := &petroRunService{cfg: config.PetroRunConfig{}, now: time.Now}
	if _, err := unavailable.CreateOrder(context.Background(), uuid.New(), validPetroRunInput()); !errors.Is(err, domain.ErrPetroRunUnavailable) {
		t.Fatalf("err = %v", err)
	}

	repo := &petroRunMemoryRepository{orders: map[uuid.UUID]entity.PetroRunOrder{}}
	input := validPetroRunInput()
	input.AcceptPrivacy = false
	if _, err := newPetroRunTestService(repo, time.Now()).CreateOrder(context.Background(), uuid.New(), input); err == nil || repo.creates != 0 {
		t.Fatalf("invalid input reached storage: err=%v creates=%d", err, repo.creates)
	}
}

func TestNormalizePetroRunWhatsapp(t *testing.T) {
	for input, want := range map[string]string{
		"081234567890":      "+6281234567890",
		"81234567890":       "+6281234567890",
		"6281234567890":     "+6281234567890",
		"+62 812-3456-7890": "+6281234567890",
		"021555123":         "",
		"0812":              "",
	} {
		got, ok := nomarlizeWhatsapp(input)
		if got != want || ok != (want != "") {
			t.Errorf("nomarlizeWhatsapp(%q) = %q, %v; want %q", input, got, ok, want)
		}
	}
}
