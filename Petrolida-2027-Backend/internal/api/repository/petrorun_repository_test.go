package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/db/migrations"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Base total sengaja tidak kelipatan 1000 (2 x 150.000 + 5.000 + 2.500).
const petroRunTestBaseTotal int64 = 307_500

func TestPetroRunNumberFormatsMatchDesign(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "order", got: petroRunOrderNumber(123), want: "PR27-000123"},
		{name: "first ticket", got: petroRunTicketNumber(1), want: "PR27-TKT-001"},
		{name: "second ticket", got: petroRunTicketNumber(2), want: "PR27-TKT-002"},
		{name: "ticket sequence over three digits", got: petroRunTicketNumber(1000), want: "PR27-TKT-1000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("number = %q, want %q", test.got, test.want)
			}
		})
	}
}

// openPetroRunTestDB membuat schema sementara supaya test tidak menyentuh data
// lokal dan bisa jalan paralel dengan test migrasi.
func openPetroRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("DB_HOST") == "" {
		t.Skip("DB_HOST is not set; skipping PostgreSQL PetroRun test")
	}

	admin := openPetroRunConnection(t, "")
	schema := fmt.Sprintf("petrorun_test_%d", time.Now().UnixNano())
	if err := admin.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schema)).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	database := openPetroRunConnection(t, schema)
	t.Cleanup(func() {
		closePetroRunConnection(database)
		_ = admin.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema)).Error
		closePetroRunConnection(admin)
	})

	if err := migrations.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func openPetroRunConnection(t *testing.T, searchPath string) *gorm.DB {
	t.Helper()
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	databaseName := os.Getenv("DB_NAME")
	if databaseName == "" {
		databaseName = "postgres"
	}
	sslMode := os.Getenv("DB_SSL_MODE")
	if sslMode == "" {
		sslMode = "disable"
	}

	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASS")),
		Host:   net.JoinHostPort(os.Getenv("DB_HOST"), port),
		Path:   databaseName,
	}
	query := databaseURL.Query()
	query.Set("sslmode", sslMode)
	if searchPath != "" {
		query.Set("search_path", searchPath)
	}
	databaseURL.RawQuery = query.Encode()

	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return database
}

func closePetroRunConnection(database *gorm.DB) {
	if sqlDatabase, err := database.DB(); err == nil {
		_ = sqlDatabase.Close()
	}
}

func newPetroRunTestOrder(now time.Time, key uuid.UUID) *entity.PetroRunOrder {
	order := &entity.PetroRunOrder{
		RegistrantName:         "Budi Santoso",
		RegistrantEmail:        "budi@mail.com",
		RegistrantPhone:        "+6281234567890",
		Status:                 entity.PetroRunWaitingPayment,
		BasePriceIDR:           150_000,
		AdminFeeIDR:            2_500,
		ParticipantSubtotalIDR: 305_000,
		BankName:               "BNI",
		BankAccountNumber:      "1234567890",
		BankAccountHolder:      "Petrolida ITS",
		ExpiresAt:              now.Add(24 * time.Hour),
		IdempotencyKey:         &key,
		Participants: []entity.PetroRunParticipant{
			{Name: "Budi Santoso", Email: "budi@mail.com", Contact: "+6281234567890", JerseySize: "2XL", MedicalHistory: "tidak ada", RunningPace: "6:00/km"},
			{Name: "Ani Lestari", Email: "budi@mail.com", Contact: "+6281234567890", JerseySize: "M", MedicalHistory: "asma", RunningPace: "7:00/km"},
		},
	}
	order.CreatedAt = now
	return order
}

func TestPetroRunCreateOrderAllocatesUniqueTotalsConcurrently(t *testing.T) {
	database := openPetroRunTestDB(t)
	repo := NewpetroRun(database)
	now := time.Now().UTC().Truncate(time.Microsecond)

	const workers = 60
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal, newPetroRunTestOrder(now, uuid.New()))
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent create: %v", err)
		}
	}

	var orders []entity.PetroRunOrder
	if err := database.Preload("Participants").Find(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if len(orders) != workers {
		t.Fatalf("orders = %d, want %d", len(orders), workers)
	}
	seen := map[int64]bool{}
	seenOrderNumbers := map[string]bool{}
	for _, order := range orders {
		if seen[order.PayableTotalIDR] {
			t.Fatalf("duplicate payable total %d", order.PayableTotalIDR)
		}
		seen[order.PayableTotalIDR] = true

		if order.UniqueCode < 1 || order.UniqueCode > 999 || order.PayableTotalIDR%1000 != int64(order.UniqueCode) {
			t.Fatalf("payable %d does not end with code %03d", order.PayableTotalIDR, order.UniqueCode)
		}
		if order.PayableTotalIDR != order.ParticipantSubtotalIDR+order.AdminFeeIDR+order.UniqueAdjustmentIDR ||
			order.UniqueAdjustmentIDR < 0 || order.UniqueAdjustmentIDR > 999 {
			t.Fatalf("payable %d != subtotal %d + admin %d + adjustment %d", order.PayableTotalIDR, order.ParticipantSubtotalIDR, order.AdminFeeIDR, order.UniqueAdjustmentIDR)
		}
		if len(order.OrderNumber) != len("PR27-000001") || !strings.HasPrefix(order.OrderNumber, "PR27-") {
			t.Fatalf("order number %q does not match PR27-000001 format", order.OrderNumber)
		}
		for _, digit := range order.OrderNumber[len("PR27-"):] {
			if digit < '0' || digit > '9' {
				t.Fatalf("order number %q contains a non-numeric sequence", order.OrderNumber)
			}
		}
		if seenOrderNumbers[order.OrderNumber] {
			t.Fatalf("duplicate order number %s", order.OrderNumber)
		}
		seenOrderNumbers[order.OrderNumber] = true
		if len(order.Participants) != 2 {
			t.Fatalf("order %s has %d participants", order.OrderNumber, len(order.Participants))
		}
	}

	var audits int64
	database.Model(&entity.PetroRunAuditLog{}).Where("action = ?", "order_created").Count(&audits)
	if audits != workers {
		t.Fatalf("audit logs = %d, want %d", audits, workers)
	}
}

func TestPetroRunCreateOrderPersistsDeadline(t *testing.T) {
	database := openPetroRunTestDB(t)
	repo := NewpetroRun(database)
	now := time.Now().UTC().Truncate(time.Microsecond)
	key := uuid.New()

	order := newPetroRunTestOrder(now, key)
	if err := repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal, order); err != nil {
		t.Fatal(err)
	}

	// Dibaca ulang lewat koneksi baru, seperti setelah server restart.
	stored, err := NewpetroRun(database).GetByIdempotencyIdKey(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.CreatedAt.Equal(now) || stored.ExpiresAt.Sub(stored.CreatedAt) != 24*time.Hour {
		t.Fatalf("created = %s, expires = %s", stored.CreatedAt, stored.ExpiresAt)
	}
	if stored.OrderNumber != order.OrderNumber || len(stored.Participants) != 2 {
		t.Fatalf("stored = %+v", stored)
	}
	if byNumber, err := repo.GetByOrderNumber(context.Background(), order.OrderNumber); err != nil || byNumber.ID != order.ID {
		t.Fatalf("get by order number = %v, %v", byNumber.ID, err)
	}
	if _, err := repo.GetByOrderNumber(context.Background(), "PR27-MISSING-001"); !errors.Is(err, domain.ErrPetroRunOrderNotFound) {
		t.Fatalf("missing order err = %v", err)
	}
}

func TestPetroRunCreateOrderRejectsReusedIdempotencyKey(t *testing.T) {
	database := openPetroRunTestDB(t)
	repo := NewpetroRun(database)
	now := time.Now().UTC().Truncate(time.Microsecond)
	key := uuid.New()

	if err := repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal, newPetroRunTestOrder(now, key)); err != nil {
		t.Fatal(err)
	}
	err := repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal, newPetroRunTestOrder(now, key))
	if !errors.Is(err, domain.ErrPetroRunConflict) {
		t.Fatalf("reused key err = %v, want conflict", err)
	}
	var count int64
	database.Model(&entity.PetroRunOrder{}).Count(&count)
	if count != 1 {
		t.Fatalf("orders = %d, want 1", count)
	}
}

func TestPetroRunCreateOrderHandlesFullCodeRange(t *testing.T) {
	database := openPetroRunTestDB(t)
	repo := NewpetroRun(database)
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Isi semua kode 001-999 untuk base total ini dengan order aktif.
	filler := make([]entity.PetroRunOrder, 0, 999)
	for code := 1; code <= 999; code++ {
		order := *newPetroRunTestOrder(now, uuid.New())
		order.Participants = nil
		order.OrderNumber = fmt.Sprintf("PR27-FILL-%03d", code)
		order.UniqueCode = code
		order.UniqueAdjustmentIDR = domain.PetroRunUniqueCode(petroRunTestBaseTotal, code)
		order.PayableTotalIDR = petroRunTestBaseTotal + order.UniqueAdjustmentIDR
		if code%2 == 0 {
			order.Status = entity.PetroRunPendingReview
		}
		filler = append(filler, order)
	}
	if err := database.CreateInBatches(filler, 200).Error; err != nil {
		t.Fatal(err)
	}

	err := repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal, newPetroRunTestOrder(now, uuid.New()))
	if !errors.Is(err, domain.ErrPetroRunExhausted) {
		t.Fatalf("full range err = %v, want exhausted", err)
	}

	// Kode 123 lewat deadline tanpa bukti: harus di-expire lalu bisa dipakai lagi.
	if err := database.Model(&entity.PetroRunOrder{}).Where("unique_code = ?", 123).
		Update("expires_at", now.Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	order := newPetroRunTestOrder(now, uuid.New())
	if err := repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal, order); err != nil {
		t.Fatalf("create after expiry: %v", err)
	}
	if order.UniqueCode != 123 {
		t.Fatalf("reused code = %d, want 123", order.UniqueCode)
	}

	// Base total lain punya rentang sendiri, jadi tetap bisa membuat order.
	other := newPetroRunTestOrder(now, uuid.New())
	if err := repo.CreateOrder(context.Background(), now, petroRunTestBaseTotal+5_000, other); err != nil {
		t.Fatalf("create with other base total: %v", err)
	}
}

func TestPetroRunExpireOverdueOnlyExpiresUnpaidOrders(t *testing.T) {
	database := openPetroRunTestDB(t)
	repo := NewpetroRun(database)
	created := time.Now().UTC().Truncate(time.Microsecond)

	cases := map[string]entity.PetroRunOrderStatus{
		"waiting": entity.PetroRunWaitingPayment,
		"review":  entity.PetroRunPendingReview,
	}
	orderNumbers := make(map[string]string, len(cases))
	for name, status := range cases {
		order := newPetroRunTestOrder(created, uuid.New())
		if err := repo.CreateOrder(context.Background(), created, petroRunTestBaseTotal, order); err != nil {
			t.Fatal(err)
		}
		orderNumbers[name] = order.OrderNumber
		if err := database.Model(order).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Satu detik sebelum deadline belum ada yang expired.
	if expired, err := repo.ExpireOverdue(context.Background(), created.Add(24*time.Hour-time.Second)); err != nil || expired != 0 {
		t.Fatalf("before deadline expired = %d, %v", expired, err)
	}
	// Tepat di deadline hanya order tanpa bukti yang expired.
	if expired, err := repo.ExpireOverdue(context.Background(), created.Add(24*time.Hour)); err != nil || expired != 1 {
		t.Fatalf("at deadline expired = %d, %v", expired, err)
	}

	// Order pending_review tetap menunggu keputusan admin walau lewat deadline.
	want := map[string]entity.PetroRunOrderStatus{"waiting": entity.PetroRunExpired, "review": entity.PetroRunPendingReview}
	for name, status := range want {
		var order entity.PetroRunOrder
		if err := database.Where("order_number = ?", orderNumbers[name]).First(&order).Error; err != nil {
			t.Fatal(err)
		}
		if order.Status != status {
			t.Fatalf("%s status = %s, want %s", name, order.Status, status)
		}
	}
}
