package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func TestPetroRunReviewRejectsStaleAttemptAndAllowsReplacement(t *testing.T) {
	db := openPetroRunTestDB(t)
	repo := NewpetroRun(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	order := newPetroRunTestOrder(now, uuid.New())
	if err := repo.CreateOrder(ctx, now, petroRunTestBaseTotal, order); err != nil {
		t.Fatal(err)
	}
	ticket := func(p entity.PetroRunParticipant, number string) (entity.PetroRunTicket, error) {
		return entity.PetroRunTicket{ID: uuid.New(), ParticipantID: p.ID, TicketNumber: number, TokenHash: uuid.NewString(), Status: "active"}, nil
	}
	submit := func(hash string) entity.PetroRunPaymentProof {
		t.Helper()
		proof := entity.PetroRunPaymentProof{FileKey: "private", FileSHA256: hash, MIMEType: "image/png", OCRClassification: "receipt"}
		record, err := repo.SubmitPaymentProof(ctx, order.OrderNumber, &proof, now, false, ticket)
		if err != nil {
			t.Fatal(err)
		}
		return record.Proof
	}
	first := submit("first")
	second := submit("second")
	if _, err := repo.ReviewPaymentProof(ctx, first.ID, true, uuid.New(), "stale", now, ticket); !errors.Is(err, domain.ErrPetroRunProofReviewed) {
		t.Fatalf("stale review=%v", err)
	}
	if _, err := repo.ReviewPaymentProof(ctx, second.ID, false, uuid.New(), "amount mismatch", now, ticket); err != nil {
		t.Fatal(err)
	}
	third := submit("replacement")
	actor := uuid.New()
	for range 2 {
		if _, err := repo.ReviewPaymentProof(ctx, third.ID, true, actor, "verified transfer", now, ticket); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := repo.GetAdminOrder(ctx, order.OrderNumber)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&entity.PetroRunTicket{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if detail.Order.Status != entity.PetroRunApproved || len(detail.Proofs) != 3 || count != 2 {
		t.Fatalf("status=%s attempts=%d tickets=%d", detail.Order.Status, len(detail.Proofs), count)
	}
}

func TestPetroRunAdminCountsGroupsAndFiltersWithoutDuplicates(t *testing.T) {
	db := openPetroRunTestDB(t)
	repo := NewpetroRun(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := newPetroRunTestOrder(now, uuid.New())
	second := newPetroRunTestOrder(now, uuid.New())
	if err := repo.CreateOrder(ctx, now, petroRunTestBaseTotal, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateOrder(ctx, now, petroRunTestBaseTotal, second); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(first).Update("status", entity.PetroRunApproved).Error; err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if err := db.Create(&entity.PetroRunPaymentProof{OrderID: first.ID, Attempt: attempt, FileKey: "private", FileSHA256: "test", MIMEType: "image/png", Status: "approved", OCRTransactionAt: &now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	q := domain.PetroRunAdminQuery{Page: 1, PageSize: 1, Sort: "created_at", Direction: "desc"}
	items, total, err := repo.ListAdminOrders(ctx, q)
	if err != nil || len(items) != 1 || total != 2 {
		t.Fatalf("list: items=%d total=%d error=%v", len(items), total, err)
	}
	summary, err := repo.SummarizeAdminOrders(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalOrders != 2 || summary.TotalParticipants != 4 || summary.Approved != 1 || summary.WaitingPayment != 1 || summary.ApprovedTotal != first.PayableTotalIDR {
		t.Fatalf("summary=%+v", summary)
	}
	from, to := now.Add(-time.Minute), now.Add(time.Minute)
	q.TransactionFrom = &from
	q.TransactionTo = &to
	items, total, err = repo.ListAdminOrders(ctx, q)
	if err != nil || total != 1 || len(items) != 1 || items[0].Order.ID != first.ID || items[0].Proofs[0].Attempt != 2 {
		t.Fatalf("transaction filter: %+v %d %v", items, total, err)
	}
	q.Search = first.Participants[0].Name
	q.Status = "approved"
	summary, err = repo.SummarizeAdminOrders(ctx, q)
	if err != nil || summary.TotalParticipants != 2 {
		t.Fatalf("filtered summary=%+v error=%v", summary, err)
	}
	detail, err := repo.GetAdminOrder(ctx, first.OrderNumber)
	if err != nil || len(detail.Proofs) != 2 || len(detail.Order.Participants) != 2 || len(detail.Audit) != 1 {
		t.Fatalf("detail=%+v error=%v", detail, err)
	}
}
