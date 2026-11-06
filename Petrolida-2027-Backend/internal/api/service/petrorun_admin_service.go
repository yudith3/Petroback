package service

import (
	"context"
	"crypto/hmac"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
)

func (s *petroRunService) GetOrder(ctx context.Context, number, token string) (domain.PetroRunOrderResponse, error) {
	if !s.cfg.Available() {
		return domain.PetroRunOrderResponse{}, domain.ErrPetroRunUnavailable
	}
	order, err := s.repository.GetByOrderNumber(ctx, normalizePetroRunCode(number))
	if err != nil {
		return domain.PetroRunOrderResponse{}, err
	}
	if !hmac.Equal([]byte(token), []byte(s.accessToken(order.ID))) {
		return domain.PetroRunOrderResponse{}, domain.ErrPetroRunInvalidAccess
	}
	if order.Status == entity.PetroRunWaitingPayment && !s.now().Before(order.ExpiresAt) {
		order.Status = entity.PetroRunExpired
	}
	return s.toResponse(order), nil
}

func (s *petroRunService) ListAdminOrders(ctx context.Context, q domain.PetroRunAdminQuery) ([]domain.PetroRunAdminOrder, int64, error) {
	if _, err := s.repository.ExpireOverdue(ctx, s.now().UTC()); err != nil {
		return nil, 0, err
	}
	records, total, err := s.repository.ListAdminOrders(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	items := make([]domain.PetroRunAdminOrder, 0, len(records))
	for _, record := range records {
		items = append(items, s.adminOrder(record, false))
	}
	return items, total, nil
}

func (s *petroRunService) GetAdminOrder(ctx context.Context, number string) (domain.PetroRunAdminOrder, error) {
	if _, err := s.repository.ExpireOverdue(ctx, s.now().UTC()); err != nil {
		return domain.PetroRunAdminOrder{}, err
	}
	record, err := s.repository.GetAdminOrder(ctx, normalizePetroRunCode(number))
	if err != nil {
		return domain.PetroRunAdminOrder{}, err
	}
	return s.adminOrder(record, true), nil
}

func (s *petroRunService) SummarizeAdminOrders(ctx context.Context, q domain.PetroRunAdminQuery) (domain.PetroRunSummary, error) {
	if _, err := s.repository.ExpireOverdue(ctx, s.now().UTC()); err != nil {
		return domain.PetroRunSummary{}, err
	}
	return s.repository.SummarizeAdminOrders(ctx, q)
}

func (s *petroRunService) adminOrder(record repository.PetroRunAdminRecord, detail bool) domain.PetroRunAdminOrder {
	order := record.Order
	public := s.toResponse(order)
	result := domain.PetroRunAdminOrder{OrderID: public.OrderID, Status: public.Status, RegistrantName: order.RegistrantName, RegistrantEmail: order.RegistrantEmail, RegistrantPhone: order.RegistrantPhone, ParticipantCount: len(order.Participants), Pricing: public.Pricing, PayableTotal: public.PayableTotal, Bank: public.Bank, CreatedAt: public.CreatedAt, ExpiresAt: public.ExpiresAt, ServerTime: public.ServerTime, VerificationSource: "none"}
	if len(record.Proofs) > 0 {
		p := record.Proofs[0]
		latest := toPaymentProofResponse(p)
		result.LatestProof = &latest
		result.VerificationSource = p.OCRSource
		if p.ReviewerID != nil {
			result.VerificationSource = "admin_manual"
		} else if p.Status == "approved" {
			result.VerificationSource = "client_ocr"
		}
	}
	if !detail {
		return result
	}
	result.Participants = make([]domain.PetroRunAdminParticipant, 0, len(order.Participants))
	for _, p := range order.Participants {
		result.Participants = append(result.Participants, domain.PetroRunAdminParticipant{ID: p.ID, Name: p.Name, JerseySize: p.JerseySize, PaceTarget: p.RunningPace, MedicalHistory: p.MedicalHistory})
	}
	result.Proofs = make([]domain.PetroRunAdminProof, 0, len(record.Proofs))
	for _, p := range record.Proofs {
		result.Proofs = append(result.Proofs, domain.PetroRunAdminProof{PetroRunPaymentProofResponse: toPaymentProofResponse(p), RawText: p.OCRRawText, Reference: p.OCRReference, OCROrderID: p.OCROrderID, FileURL: "/api/v1/petrorun/admin/payment-proofs/" + p.ID.String() + "/file", ReviewerID: p.ReviewerID, ReviewedAt: p.ReviewedAt, ReviewNote: p.ReviewNote})
	}
	result.Tickets = s.toTicketsResponse(order).Tickets
	result.Audit = make([]domain.PetroRunAdminAudit, 0, len(record.Audit))
	for _, a := range record.Audit {
		result.Audit = append(result.Audit, domain.PetroRunAdminAudit{ID: a.ID, ActorID: a.ActorID, Action: a.Action, FromStatus: a.FromStatus, ToStatus: a.ToStatus, Reason: strings.TrimSpace(a.Reason), CreatedAt: a.CreatedAt})
	}
	return result
}
