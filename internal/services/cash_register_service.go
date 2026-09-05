package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCashRegister = errors.New("invalid cash register data")

type CashRegisterService struct {
	repository *repositories.CashRegisterRepository
	now        func() time.Time
}

func NewCashRegisterService(repository *repositories.CashRegisterRepository) *CashRegisterService {
	return &CashRegisterService{repository: repository, now: time.Now}
}

func (s *CashRegisterService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string,
	filter models.CashRegisterFilter, offset, limit int) ([]models.CashRegister, int, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil {
		return nil, 0, err
	}
	filter.Query = strings.Join(strings.Fields(filter.Query), " ")
	if len([]rune(filter.Query)) > 80 || !validCashRegisterStatus(filter.Status, true) {
		return nil, 0, ErrInvalidCashRegister
	}
	if err = s.ensureLibraryAccess(ctx, claims, libraryID); err != nil {
		return nil, 0, err
	}
	if offset < 0 { offset = 0 }
	if limit < 1 { limit = 30 }
	if limit > 100 { limit = 100 }
	return s.repository.List(ctx, libraryID, filter, offset, limit)
}

func (s *CashRegisterService) Find(ctx context.Context, claims *auth.Claims, id string) (models.CashRegister, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil || strings.TrimSpace(id) == "" {
		if err != nil { return models.CashRegister{}, err }
		return models.CashRegister{}, ErrInvalidCashRegister
	}
	if err = s.ensureLibraryAccess(ctx, claims, libraryID); err != nil {
		return models.CashRegister{}, err
	}
	return s.repository.Find(ctx, id, libraryID)
}

func (s *CashRegisterService) Create(ctx context.Context, claims *auth.Claims,
	input models.CashRegisterInput) (models.CashRegister, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil { return models.CashRegister{}, err }
	name, normalized, err := normalizeCashRegisterName(input.Name)
	if err != nil { return models.CashRegister{}, err }
	if err = s.ensureLibraryActive(ctx, libraryID); err != nil { return models.CashRegister{}, err }
	id, err := identity.NewID()
	if err != nil { return models.CashRegister{}, err }
	auditID, err := identity.NewID()
	if err != nil { return models.CashRegister{}, err }
	now := s.now().UTC().Format(time.RFC3339Nano)
	register := models.CashRegister{ID: id, LibraryID: libraryID, Name: name,
		Status: models.CashRegisterStatusActive, Version: 1, CreatedBy: claims.Subject,
		CreatedAt: now, UpdatedAt: now}
	snapshot, err := cashRegisterSnapshot(register)
	if err != nil { return models.CashRegister{}, err }
	if err = s.repository.Create(ctx, register, normalized, claims.Subject, auditID, snapshot); err != nil {
		return models.CashRegister{}, err
	}
	return register, nil
}

func (s *CashRegisterService) Update(ctx context.Context, claims *auth.Claims, id string,
	input models.CashRegisterInput) (models.CashRegister, error) {
	existing, err := s.Find(ctx, claims, id)
	if err != nil { return models.CashRegister{}, err }
	if input.LibraryID != "" && strings.TrimSpace(input.LibraryID) != existing.LibraryID {
		return models.CashRegister{}, ErrBookForbidden
	}
	if input.Version < 1 { return models.CashRegister{}, ErrInvalidCashRegister }
	name, normalized, err := normalizeCashRegisterName(input.Name)
	if err != nil { return models.CashRegister{}, err }
	updated := existing
	updated.Name, updated.Version = name, existing.Version+1
	updated.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	oldValues, err := cashRegisterSnapshot(existing)
	if err != nil { return models.CashRegister{}, err }
	newValues, err := cashRegisterSnapshot(updated)
	if err != nil { return models.CashRegister{}, err }
	auditID, err := identity.NewID()
	if err != nil { return models.CashRegister{}, err }
	if err = s.repository.Update(ctx, updated, normalized, input.Version, claims.Subject,
		auditID, oldValues, newValues); err != nil { return models.CashRegister{}, err }
	return updated, nil
}

func (s *CashRegisterService) Disable(ctx context.Context, claims *auth.Claims, id string, version int) error {
	return s.changeStatus(ctx, claims, id, version, models.CashRegisterStatusActive, models.CashRegisterStatusDisabled)
}

func (s *CashRegisterService) Reactivate(ctx context.Context, claims *auth.Claims, id string, version int) error {
	return s.changeStatus(ctx, claims, id, version, models.CashRegisterStatusDisabled, models.CashRegisterStatusActive)
}

func (s *CashRegisterService) changeStatus(ctx context.Context, claims *auth.Claims, id string, version int,
	expected, next models.CashRegisterStatus) error {
	if version < 1 { return ErrInvalidCashRegister }
	register, err := s.Find(ctx, claims, id)
	if err != nil { return err }
	if register.Status != expected { return repositories.ErrCashRegisterState }
	auditID, err := identity.NewID()
	if err != nil { return err }
	return s.repository.ChangeStatus(ctx, register, expected, next, version, claims.Subject, auditID,
		s.now().UTC().Format(time.RFC3339Nano))
}

func (s *CashRegisterService) ensureLibraryActive(ctx context.Context, libraryID string) error {
	active, err := s.repository.LibraryActive(ctx, libraryID)
	if err != nil { return err }
	if !active { return repositories.ErrLibraryUnavailable }
	return nil
}

func (s *CashRegisterService) ensureLibraryAccess(ctx context.Context, claims *auth.Claims, libraryID string) error {
	if claims.Role != models.RoleOwnerLibrary { return nil }
	if err := s.ensureLibraryActive(ctx, libraryID); err != nil { return ErrBookForbidden }
	return nil
}

func normalizeCashRegisterName(value string) (string, string, error) {
	name := strings.Join(strings.Fields(value), " ")
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 {
		return "", "", ErrInvalidCashRegister
	}
	return name, strings.ToLower(name), nil
}

func validCashRegisterStatus(status models.CashRegisterStatus, empty bool) bool {
	return (empty && status == "") || status == models.CashRegisterStatusActive ||
		status == models.CashRegisterStatusDisabled
}

func cashRegisterSnapshot(register models.CashRegister) (string, error) {
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
		Status models.CashRegisterStatus `json:"status"`
		Version int `json:"version"`
	}{register.Name, register.Status, register.Version})
	return string(payload), err
}
