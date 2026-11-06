package service

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/config"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

func validPetroRunOCR(now time.Time) domain.PetroRunOCRInput {
	confidence := 0.95
	amount := int64(307503)
	jakarta := time.FixedZone("WIB", 7*60*60)
	return domain.PetroRunOCRInput{
		Classification:     "receipt",
		RawText:            "Transfer berhasil",
		SenderName:         "Ayu Runner",
		RecipientName:      "Petrolida ITS",
		TransactionStatus:  "success",
		ExtractedAmount:    &amount,
		DestinationAccount: "1234567890",
		DestinationBank:    "BNI",
		TransactionAt:      now.Add(-time.Minute).In(jakarta).Format(time.RFC3339),
		Timezone:           "Asia/Jakarta",
		Precision:          "second",
		Confidence:         &confidence,
		OrderID:            "PR27-000001",
	}
}

func TestEvaluatePaymentProofOCRRequiresExactOrderPaymentMatch(t *testing.T) {
	now := time.Date(2027, time.January, 1, 10, 0, 0, 0, time.UTC)
	order := entity.PetroRunOrder{
		OrderNumber:       "PR27-000001",
		PayableTotalIDR:   307503,
		BankName:          "BNI",
		BankAccountNumber: "1234567890",
		BankAccountHolder: "Petrolida ITS",
		ExpiresAt:         now.Add(23 * time.Hour),
	}
	order.Timestamp.CreatedAt = now.Add(-time.Hour)

	t.Run("exact match can auto approve", func(t *testing.T) {
		_, transactionAt, reasons, err := evaluatePaymentProofOCR(validPetroRunOCR(now), order, now)
		if err != nil {
			t.Fatal(err)
		}
		if transactionAt == nil || len(reasons) != 0 {
			t.Fatalf("transactionAt = %v, reasons = %v; want timestamp and no mismatch", transactionAt, reasons)
		}
	})

	tests := []struct {
		name   string
		change func(*domain.PetroRunOCRInput)
		reason string
	}{
		{"unique amount mismatch", func(input *domain.PetroRunOCRInput) { *input.ExtractedAmount++ }, "amount_mismatch"},
		{"wrong order", func(input *domain.PetroRunOCRInput) { input.OrderID = "PR27-000002" }, "order_id_mismatch"},
		{"missing OCR text", func(input *domain.PetroRunOCRInput) { input.RawText = "" }, "ocr_raw_text_missing"},
		{"failed transaction", func(input *domain.PetroRunOCRInput) { input.TransactionStatus = "failed" }, "transaction_failed"},
		{"unknown transaction status", func(input *domain.PetroRunOCRInput) { input.TransactionStatus = "unknown" }, "transaction_status_unknown"},
		{"wrong recipient name", func(input *domain.PetroRunOCRInput) { input.RecipientName = "Other Recipient" }, "recipient_name_mismatch"},
		{"low confidence", func(input *domain.PetroRunOCRInput) { *input.Confidence = 0.89 }, "low_or_missing_ocr_confidence"},
		{"timezone mismatch", func(input *domain.PetroRunOCRInput) { input.Timezone = "UTC" }, "transaction_timezone_mismatch"},
		{"timestamp outside window", func(input *domain.PetroRunOCRInput) {
			input.TransactionAt = now.Add(-2 * time.Hour).In(time.FixedZone("WIB", 7*60*60)).Format(time.RFC3339)
		}, "transaction_outside_payment_window"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validPetroRunOCR(now)
			test.change(&input)
			_, _, reasons, err := evaluatePaymentProofOCR(input, order, now)
			if err != nil {
				t.Fatal(err)
			}
			if !containsProofReason(reasons, test.reason) {
				t.Fatalf("reasons = %v, want %q", reasons, test.reason)
			}
		})
	}
}

func TestValidatePaymentProofFileChecksExtensionAndSignature(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
		wantMIME string
		wantErr  bool
	}{
		{"jpeg", "proof.jpg", []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 'J', 'F', 'I', 'F', 0}, "image/jpeg", false},
		{"png", "proof.png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, "image/png", false},
		{"pdf", "proof.pdf", []byte("%PDF-1.7\nproof"), "application/pdf", false},
		{"wrong extension", "proof.jpg", []byte("%PDF-1.7\nproof"), "", true},
		{"unsupported extension", "proof.gif", []byte("GIF89a"), "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validatePaymentProofFile(test.filename, test.content)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePaymentProofFile() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.wantMIME {
				t.Errorf("MIME type = %q, want %q", got, test.wantMIME)
			}
		})
	}

	if _, err := validatePaymentProofFile("proof.png", make([]byte, maxPaymentProofBytes+1)); err == nil {
		t.Fatalf("oversized file error = %v, want size validation", err)
	}
}

func containsProofReason(reasons []string, expected string) bool {
	for _, reason := range reasons {
		if reason == expected {
			return true
		}
	}
	return false
}

type paymentProofTestRepository struct {
	repository.PetroRunRepository
	order       entity.PetroRunOrder
	autoApprove bool
	proof       entity.PetroRunPaymentProof
}

func (r *paymentProofTestRepository) GetByOrderNumber(_ context.Context, _ string) (entity.PetroRunOrder, error) {
	return r.order, nil
}

func (r *paymentProofTestRepository) SubmitPaymentProof(
	_ context.Context,
	_ string,
	proof *entity.PetroRunPaymentProof,
	now time.Time,
	autoApprove bool,
	_ func(entity.PetroRunParticipant, string) (entity.PetroRunTicket, error),
) (repository.PetroRunProofRecord, error) {
	proof.ID = uuid.New()
	proof.Attempt = 1
	proof.CreatedAt = now
	proof.Status = "pending_review"
	if autoApprove {
		proof.Status = "approved"
	}
	r.autoApprove = autoApprove
	r.proof = *proof
	return repository.PetroRunProofRecord{Proof: *proof, Order: r.order}, nil
}

type paymentProofTestStore struct {
	savedName string
	savedData string
	deleted   bool
}

func (s *paymentProofTestStore) Save(name string, _ int64, content io.Reader) (string, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return "", err
	}
	s.savedName = name
	s.savedData = string(data)
	return "proof-key", nil
}

func (*paymentProofTestStore) Open(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("proof")), nil
}

func (s *paymentProofTestStore) Delete(string) error {
	s.deleted = true
	return nil
}

func TestSubmitPaymentProofPersistsOriginalAndUsesExactOCRMatch(t *testing.T) {
	now := time.Date(2027, time.January, 1, 10, 0, 0, 0, time.UTC)
	order := entity.PetroRunOrder{
		ID:                uuid.New(),
		OrderNumber:       "PR27-000001",
		Status:            entity.PetroRunWaitingPayment,
		PayableTotalIDR:   307503,
		BankName:          "BNI",
		BankAccountNumber: "1234567890",
		BankAccountHolder: "Petrolida ITS",
		ExpiresAt:         now.Add(23 * time.Hour),
	}
	order.Timestamp.CreatedAt = now.Add(-time.Hour)
	repo := &paymentProofTestRepository{order: order}
	store := &paymentProofTestStore{}
	cfg := config.PetroRunConfig{
		BasePriceIDR:      150000,
		BankName:          order.BankName,
		BankAccountNumber: order.BankAccountNumber,
		BankAccountHolder: "Petrolida ITS",
		AccessSecret:      "test-secret-at-least-32-characters-long",
	}
	proofService := &petroRunService{repository: repo, cfg: cfg, proofStore: store, now: func() time.Time { return now }}
	content := []byte("%PDF-1.7\noriginal proof")

	result, err := proofService.SubmitPaymentProof(context.Background(), order.OrderNumber, proofService.accessToken(order.ID), validPetroRunOCR(now), "proof.pdf", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "approved" || result.OrderID != order.OrderNumber || !repo.autoApprove {
		t.Fatalf("response = %+v, autoApprove = %v", result, repo.autoApprove)
	}
	if store.savedData != string(content) || store.savedName == "" || repo.proof.OCRSource != "browser_client" {
		t.Fatalf("proof storage or source metadata incorrect: store=%+v proof=%+v", store, repo.proof)
	}
	if repo.proof.OCRSenderName != "Ayu Runner" ||
		repo.proof.OCRRecipientName != "Petrolida ITS" ||
		repo.proof.OCRTransactionStatus != "success" ||
		result.SenderName != "Ayu Runner" ||
		result.RecipientName != "Petrolida ITS" ||
		result.TransactionStatus != "success" {
		t.Fatalf("transaction parties/status not persisted and returned: proof=%+v response=%+v", repo.proof, result)
	}

	ocr := validPetroRunOCR(now)
	*ocr.ExtractedAmount++
	if _, err := proofService.SubmitPaymentProof(context.Background(), order.OrderNumber, proofService.accessToken(order.ID), ocr, "proof.pdf", content); err != nil {
		t.Fatal(err)
	}
	if repo.autoApprove || !strings.Contains(repo.proof.OCRMismatchReasons, "amount_mismatch") {
		t.Fatalf("mismatched proof auto-approved: %+v", repo.proof)
	}
}
