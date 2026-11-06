package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/google/uuid"
)

type resumePetroRunRepo struct {
	repository.PetroRunRepository
	order entity.PetroRunOrder
}

func (r resumePetroRunRepo) GetByOrderNumber(context.Context, string) (entity.PetroRunOrder, error) {
	return r.order, nil
}

func TestPetroRunResumeRequiresTokenAndUsesOriginalDeadline(t *testing.T) {
	now := time.Now().UTC()
	order := entity.PetroRunOrder{ID: uuid.New(), OrderNumber: "PR27-000001", Status: entity.PetroRunWaitingPayment, ExpiresAt: now, RegistrantEmail: "private@example.com", Participants: []entity.PetroRunParticipant{{MedicalHistory: "private medical history"}}}
	svc := newPetroRunTestService(resumePetroRunRepo{order: order}, now)
	if _, err := svc.GetOrder(context.Background(), order.OrderNumber, "wrong-token"); !errors.Is(err, domain.ErrPetroRunInvalidAccess) {
		t.Fatalf("wrong-token err=%v", err)
	}
	result, err := svc.GetOrder(context.Background(), order.OrderNumber, svc.accessToken(order.ID))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "expired" || !result.ExpiresAt.Equal(order.ExpiresAt) {
		t.Fatalf("deadline changed: %+v", result)
	}
	data, _ := json.Marshal(result)
	for _, secret := range []string{order.RegistrantEmail, "private medical history", "registrant_email", "medical_history"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("resume response leaked %s", secret)
		}
	}
}
