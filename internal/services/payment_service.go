package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

var ErrInvalidPayment = errors.New("invalid payment data")

type PaymentService struct {
	repository *repositories.PaymentRepository
	now        func() time.Time
}

func NewPaymentService(repository *repositories.PaymentRepository) *PaymentService {
	return &PaymentService{repository: repository, now: time.Now}
}

func (s *PaymentService) List(ctx context.Context, claims *auth.Claims, saleID string,
	filter models.PaymentFilter, offset, limit int) ([]models.Payment, int, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return nil, 0, err }
	if strings.TrimSpace(saleID) == "" || !validPaymentMethod(filter.Method, true) || !validPaymentStatus(filter.Status, true) {
		return nil, 0, ErrInvalidPayment
	}
	if _, err = s.repository.SaleLibrary(ctx, saleID, libraryID); err != nil { return nil, 0, err }
	if offset < 0 { offset = 0 }
	if limit < 1 { limit = 30 }
	if limit > 100 { limit = 100 }
	return s.repository.List(ctx, saleID, libraryID, filter, offset, limit)
}

func (s *PaymentService) Balance(ctx context.Context, claims *auth.Claims, saleID string) (models.SalePaymentBalance, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return models.SalePaymentBalance{}, err }
	if strings.TrimSpace(saleID) == "" { return models.SalePaymentBalance{}, ErrInvalidPayment }
	return s.repository.Balance(ctx, saleID, libraryID)
}

func (s *PaymentService) Create(ctx context.Context, claims *auth.Claims, saleID string,
	input models.PaymentInput) (models.Payment, error) {
	libraryScope, err := resolveBookScope(claims, "", false)
	if err != nil { return models.Payment{}, err }
	saleID = strings.TrimSpace(saleID)
	input.CashRegisterID = strings.TrimSpace(input.CashRegisterID)
	input.ExternalReference = strings.TrimSpace(input.ExternalReference)
	input.Notes = strings.TrimSpace(input.Notes)
	if saleID == "" || input.CashRegisterID == "" || !validPaymentMethod(input.Method, false) ||
		math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) || input.Amount <= 0 ||
		len([]rune(input.ExternalReference)) > 160 || len([]rune(input.Notes)) > 1000 {
		return models.Payment{}, ErrInvalidPayment
	}
	libraryID, err := s.repository.SaleLibrary(ctx, saleID, libraryScope)
	if err != nil { return models.Payment{}, err }
	id, err := identity.NewID()
	if err != nil { return models.Payment{}, err }
	auditID, err := identity.NewID()
	if err != nil { return models.Payment{}, err }
	now := s.now().UTC().Format(time.RFC3339Nano)
	payment := models.Payment{ID: id, LibraryID: libraryID, SaleID: saleID,
		CashRegisterID: input.CashRegisterID, Method: input.Method, Amount: input.Amount,
		ExternalReference: input.ExternalReference, Notes: input.Notes,
		Status: models.PaymentStatusRecorded, Version: 1, RecordedBy: claims.Subject,
		CreatedAt: now, UpdatedAt: now}
	snapshot, err := paymentSnapshot(payment)
	if err != nil { return models.Payment{}, err }
	if err = s.repository.Create(ctx, payment, auditID, snapshot); err != nil { return models.Payment{}, err }
	return payment, nil
}

func (s *PaymentService) Void(ctx context.Context, claims *auth.Claims, id string,
	input models.PaymentVoidInput) (models.Payment, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return models.Payment{}, err }
	input.Reason = strings.Join(strings.Fields(input.Reason), " ")
	if strings.TrimSpace(id) == "" || input.Version < 1 || len([]rune(input.Reason)) < 3 || len([]rune(input.Reason)) > 500 {
		return models.Payment{}, ErrInvalidPayment
	}
	payment, err := s.repository.Find(ctx, id, libraryID)
	if err != nil { return models.Payment{}, err }
	if payment.Status != models.PaymentStatusRecorded { return models.Payment{}, repositories.ErrPaymentState }
	updated := payment
	updated.Status, updated.Version = models.PaymentStatusVoided, payment.Version+1
	updated.VoidedBy = claims.Subject
	updated.VoidedAt = s.now().UTC().Format(time.RFC3339Nano)
	updated.UpdatedAt = updated.VoidedAt
	if updated.Notes == "" { updated.Notes = input.Reason } else { updated.Notes += " | " + input.Reason }
	oldValues, err := paymentSnapshot(payment)
	if err != nil { return models.Payment{}, err }
	newValues, err := paymentSnapshot(updated)
	if err != nil { return models.Payment{}, err }
	auditID, err := identity.NewID()
	if err != nil { return models.Payment{}, err }
	if err = s.repository.Void(ctx, payment, input.Version, input.Reason, claims.Subject, auditID,
		oldValues, newValues, updated.UpdatedAt); err != nil { return models.Payment{}, err }
	return updated, nil
}

func validPaymentMethod(method models.PaymentMethod, empty bool) bool {
	return (empty && method == "") || method == models.PaymentMethodCash ||
		method == models.PaymentMethodMobileMoney || method == models.PaymentMethodCard
}

func validPaymentStatus(status models.PaymentStatus, empty bool) bool {
	return (empty && status == "") || status == models.PaymentStatusRecorded || status == models.PaymentStatusVoided
}

func paymentSnapshot(payment models.Payment) (string, error) {
	payload, err := json.Marshal(struct {
		SaleID string `json:"saleId"`
		CashRegisterID string `json:"cashRegisterId"`
		Method models.PaymentMethod `json:"method"`
		Amount float64 `json:"amount"`
		ExternalReference string `json:"externalReference,omitempty"`
		Notes string `json:"notes,omitempty"`
		Status models.PaymentStatus `json:"status"`
		Version int `json:"version"`
	}{payment.SaleID, payment.CashRegisterID, payment.Method, payment.Amount,
		payment.ExternalReference, payment.Notes, payment.Status, payment.Version})
	return string(payload), err
}
