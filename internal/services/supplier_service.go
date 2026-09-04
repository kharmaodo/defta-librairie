package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"
)

var ErrInvalidSupplier = errors.New("invalid supplier data")

type SupplierService struct {
	repository *repositories.SupplierRepository
	now func() time.Time
}

func NewSupplierService(repository *repositories.SupplierRepository) *SupplierService {
	return &SupplierService{repository: repository, now: time.Now}
}

func (s *SupplierService) List(ctx context.Context, claims *auth.Claims, requestedLibrary, query string,
	status models.SupplierStatus, offset, limit int) ([]models.Supplier, int, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil { return nil, 0, err }
	query = strings.Join(strings.Fields(query), " ")
	if len([]rune(query)) > 160 || !validSupplierStatus(status, true) { return nil, 0, ErrInvalidSupplier }
	if err = s.ensureOwnerLibraryActive(ctx, claims, libraryID); err != nil { return nil, 0, err }
	if offset < 0 { offset = 0 }
	if limit < 1 { limit = 30 }
	if limit > 100 { limit = 100 }
	return s.repository.List(ctx, libraryID, query, status, offset, limit)
}

func (s *SupplierService) Find(ctx context.Context, claims *auth.Claims, id string) (models.Supplier, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return models.Supplier{}, err }
	if strings.TrimSpace(id) == "" { return models.Supplier{}, ErrInvalidSupplier }
	if err = s.ensureOwnerLibraryActive(ctx, claims, libraryID); err != nil { return models.Supplier{}, err }
	return s.repository.Find(ctx, id, libraryID)
}

func (s *SupplierService) Create(ctx context.Context, claims *auth.Claims, input models.SupplierInput) (models.Supplier, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil { return models.Supplier{}, err }
	input.LibraryID = libraryID
	if err = normalizeSupplier(&input, false); err != nil { return models.Supplier{}, err }
	if err = s.ensureLibraryActive(ctx, libraryID); err != nil { return models.Supplier{}, err }
	id, auditID, err := supplierIDs()
	if err != nil { return models.Supplier{}, err }
	now := s.now().UTC().Format(time.RFC3339Nano)
	supplier := models.Supplier{ID:id, LibraryID:libraryID, Name:input.Name, ContactName:input.ContactName,
		Phone:input.Phone, Email:input.Email, Address:input.Address, Status:models.SupplierStatusActive,
		Version:1, CreatedBy:claims.Subject, CreatedAt:now, UpdatedAt:now}
	snapshot, err := supplierSnapshot(supplier)
	if err != nil { return models.Supplier{}, err }
	if err = s.repository.Create(ctx, supplier, strings.ToLower(supplier.Name), claims.Subject, auditID, snapshot); err != nil { return models.Supplier{}, err }
	return supplier, nil
}

func (s *SupplierService) Update(ctx context.Context, claims *auth.Claims, id string, input models.SupplierInput) (models.Supplier, error) {
	existing, err := s.Find(ctx, claims, id)
	if err != nil { return models.Supplier{}, err }
	if input.LibraryID != "" && strings.TrimSpace(input.LibraryID) != existing.LibraryID { return models.Supplier{}, ErrBookForbidden }
	if err = normalizeSupplier(&input, true); err != nil { return models.Supplier{}, err }
	updated := existing
	updated.Name, updated.ContactName, updated.Phone, updated.Email, updated.Address = input.Name, input.ContactName, input.Phone, input.Email, input.Address
	updated.Version, updated.UpdatedAt = existing.Version+1, s.now().UTC().Format(time.RFC3339Nano)
	oldValues, err := supplierSnapshot(existing); if err != nil { return models.Supplier{}, err }
	newValues, err := supplierSnapshot(updated); if err != nil { return models.Supplier{}, err }
	_, auditID, err := supplierIDs(); if err != nil { return models.Supplier{}, err }
	if err = s.repository.Update(ctx, updated, input.Version, claims.Subject, auditID, oldValues, newValues); err != nil { return models.Supplier{}, err }
	return updated, nil
}

func (s *SupplierService) Disable(ctx context.Context, claims *auth.Claims, id string, version int) error {
	return s.changeStatus(ctx, claims, id, version, models.SupplierStatusActive, models.SupplierStatusDisabled)
}

func (s *SupplierService) Reactivate(ctx context.Context, claims *auth.Claims, id string, version int) error {
	return s.changeStatus(ctx, claims, id, version, models.SupplierStatusDisabled, models.SupplierStatusActive)
}

func (s *SupplierService) changeStatus(ctx context.Context, claims *auth.Claims, id string, version int,
	expected, next models.SupplierStatus) error {
	if version < 1 { return ErrInvalidSupplier }
	supplier, err := s.Find(ctx, claims, id)
	if err != nil { return err }
	if supplier.Status != expected { return repositories.ErrSupplierState }
	_, auditID, err := supplierIDs(); if err != nil { return err }
	return s.repository.ChangeStatus(ctx, supplier, expected, next, version, claims.Subject, auditID,
		s.now().UTC().Format(time.RFC3339Nano))
}

func normalizeSupplier(input *models.SupplierInput, requireVersion bool) error {
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	input.ContactName = strings.Join(strings.Fields(input.ContactName), " ")
	input.Phone, input.Email, input.Address = strings.TrimSpace(input.Phone), strings.ToLower(strings.TrimSpace(input.Email)), strings.TrimSpace(input.Address)
	if len([]rune(input.Name)) < 2 || len([]rune(input.Name)) > 160 || len([]rune(input.ContactName)) > 160 ||
		len([]rune(input.Phone)) > 40 || len([]rune(input.Email)) > 254 || len([]rune(input.Address)) > 500 ||
		(requireVersion && input.Version < 1) { return ErrInvalidSupplier }
	if input.Email != "" { if _, err := mail.ParseAddress(input.Email); err != nil { return ErrInvalidSupplier } }
	return nil
}

func validSupplierStatus(status models.SupplierStatus, empty bool) bool {
	return (empty && status == "") || status == models.SupplierStatusActive || status == models.SupplierStatusDisabled
}

func (s *SupplierService) ensureLibraryActive(ctx context.Context, libraryID string) error {
	active, err := s.repository.LibraryActive(ctx, libraryID)
	if err != nil { return err }
	if !active { return repositories.ErrLibraryUnavailable }
	return nil
}

func (s *SupplierService) ensureOwnerLibraryActive(ctx context.Context, claims *auth.Claims, libraryID string) error {
	if claims.Role != models.RoleOwnerLibrary { return nil }
	if err := s.ensureLibraryActive(ctx, libraryID); err != nil { return ErrBookForbidden }
	return nil
}

func supplierIDs() (string, string, error) {
	id, err := identity.NewID(); if err != nil { return "", "", err }
	auditID, err := identity.NewID(); return id, auditID, err
}

func supplierSnapshot(supplier models.Supplier) (string, error) {
	payload, err := json.Marshal(struct {
		Name string `json:"name"`; ContactName string `json:"contactName,omitempty"`; Phone string `json:"phone,omitempty"`
		Email string `json:"email,omitempty"`; Address string `json:"address,omitempty"`; Status models.SupplierStatus `json:"status"`; Version int `json:"version"`
	}{supplier.Name, supplier.ContactName, supplier.Phone, supplier.Email, supplier.Address, supplier.Status, supplier.Version})
	return string(payload), err
}
