package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

var ErrInvalidCustomer = errors.New("invalid customer data")

type CustomerService struct {
	repository *repositories.CustomerRepository
	now        func() time.Time
}

func NewCustomerService(repository *repositories.CustomerRepository) *CustomerService {
	return &CustomerService{repository: repository, now: time.Now}
}

func (s *CustomerService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string,
	filter models.CustomerFilter, offset, limit int) ([]models.Customer, int, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil {
		return nil, 0, err
	}
	filter.Query = strings.Join(strings.Fields(filter.Query), " ")
	if len([]rune(filter.Query)) > 160 || !validCustomerStatus(filter.Status, true) {
		return nil, 0, ErrInvalidCustomer
	}
	if err = s.ensureOwnerLibraryActive(ctx, claims, libraryID); err != nil {
		return nil, 0, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	return s.repository.List(ctx, libraryID, filter, offset, limit)
}

func (s *CustomerService) Find(ctx context.Context, claims *auth.Claims, id string) (models.Customer, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.Customer{}, err
	}
	if strings.TrimSpace(id) == "" {
		return models.Customer{}, ErrInvalidCustomer
	}
	if err = s.ensureOwnerLibraryActive(ctx, claims, libraryID); err != nil {
		return models.Customer{}, err
	}
	return s.repository.Find(ctx, id, libraryID)
}

func (s *CustomerService) Create(ctx context.Context, claims *auth.Claims, input models.CustomerInput) (models.Customer, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil {
		return models.Customer{}, err
	}
	if err = normalizeCustomer(&input, false); err != nil {
		return models.Customer{}, err
	}
	if err = s.ensureLibraryActive(ctx, libraryID); err != nil {
		return models.Customer{}, err
	}
	id, auditID, err := customerIDs()
	if err != nil {
		return models.Customer{}, err
	}
	now := s.now().UTC()
	reference := fmt.Sprintf("C-%s-%s", now.Format("20060102"), strings.ToUpper(strings.ReplaceAll(id, "-", "")[:8]))
	timestamp := now.Format(time.RFC3339Nano)
	customer := models.Customer{ID: id, LibraryID: libraryID, Reference: reference, Name: input.Name,
		Phone: input.Phone, Email: input.Email, Address: input.Address, Notes: input.Notes,
		Status: models.CustomerStatusActive, Version: 1, CreatedBy: claims.Subject,
		CreatedAt: timestamp, UpdatedAt: timestamp}
	snapshot, err := customerSnapshot(customer)
	if err != nil {
		return models.Customer{}, err
	}
	if err = s.repository.Create(ctx, customer, claims.Subject, auditID, snapshot); err != nil {
		return models.Customer{}, err
	}
	return customer, nil
}

func (s *CustomerService) Update(ctx context.Context, claims *auth.Claims, id string, input models.CustomerInput) (models.Customer, error) {
	existing, err := s.Find(ctx, claims, id)
	if err != nil {
		return models.Customer{}, err
	}
	if input.LibraryID != "" && strings.TrimSpace(input.LibraryID) != existing.LibraryID {
		return models.Customer{}, ErrBookForbidden
	}
	if err = normalizeCustomer(&input, true); err != nil {
		return models.Customer{}, err
	}
	updated := existing
	updated.Name, updated.Phone, updated.Email = input.Name, input.Phone, input.Email
	updated.Address, updated.Notes = input.Address, input.Notes
	updated.Version, updated.UpdatedAt = existing.Version+1, s.now().UTC().Format(time.RFC3339Nano)
	oldValues, err := customerSnapshot(existing)
	if err != nil {
		return models.Customer{}, err
	}
	newValues, err := customerSnapshot(updated)
	if err != nil {
		return models.Customer{}, err
	}
	_, auditID, err := customerIDs()
	if err != nil {
		return models.Customer{}, err
	}
	if err = s.repository.Update(ctx, updated, input.Version, claims.Subject, auditID, oldValues, newValues); err != nil {
		return models.Customer{}, err
	}
	return updated, nil
}

func (s *CustomerService) Disable(ctx context.Context, claims *auth.Claims, id string, version int) error {
	return s.changeStatus(ctx, claims, id, version, models.CustomerStatusActive, models.CustomerStatusDisabled)
}

func (s *CustomerService) Reactivate(ctx context.Context, claims *auth.Claims, id string, version int) error {
	return s.changeStatus(ctx, claims, id, version, models.CustomerStatusDisabled, models.CustomerStatusActive)
}

func (s *CustomerService) changeStatus(ctx context.Context, claims *auth.Claims, id string, version int,
	expected, next models.CustomerStatus) error {
	if version < 1 {
		return ErrInvalidCustomer
	}
	customer, err := s.Find(ctx, claims, id)
	if err != nil {
		return err
	}
	if customer.Status != expected {
		return repositories.ErrCustomerState
	}
	_, auditID, err := customerIDs()
	if err != nil {
		return err
	}
	return s.repository.ChangeStatus(ctx, customer, expected, next, version, claims.Subject, auditID,
		s.now().UTC().Format(time.RFC3339Nano))
}

func normalizeCustomer(input *models.CustomerInput, requireVersion bool) error {
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	input.Phone = strings.Join(strings.Fields(input.Phone), " ")
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Address, input.Notes = strings.TrimSpace(input.Address), strings.TrimSpace(input.Notes)
	if len([]rune(input.Name)) < 2 || len([]rune(input.Name)) > 160 || len([]rune(input.Phone)) > 40 ||
		len([]rune(input.Email)) > 254 || len([]rune(input.Address)) > 500 || len([]rune(input.Notes)) > 1000 ||
		(requireVersion && input.Version < 1) {
		return ErrInvalidCustomer
	}
	if input.Email != "" {
		if _, err := mail.ParseAddress(input.Email); err != nil {
			return ErrInvalidCustomer
		}
	}
	return nil
}

func validCustomerStatus(status models.CustomerStatus, empty bool) bool {
	return (empty && status == "") || status == models.CustomerStatusActive || status == models.CustomerStatusDisabled
}

func (s *CustomerService) ensureLibraryActive(ctx context.Context, libraryID string) error {
	active, err := s.repository.LibraryActive(ctx, libraryID)
	if err != nil {
		return err
	}
	if !active {
		return repositories.ErrLibraryUnavailable
	}
	return nil
}

func (s *CustomerService) ensureOwnerLibraryActive(ctx context.Context, claims *auth.Claims, libraryID string) error {
	if claims.Role != models.RoleOwnerLibrary {
		return nil
	}
	if err := s.ensureLibraryActive(ctx, libraryID); err != nil {
		return ErrBookForbidden
	}
	return nil
}

func customerIDs() (string, string, error) {
	id, err := identity.NewID()
	if err != nil {
		return "", "", err
	}
	auditID, err := identity.NewID()
	return id, auditID, err
}

func customerSnapshot(customer models.Customer) (string, error) {
	payload, err := json.Marshal(struct {
		Reference string                `json:"reference"`
		Name      string                `json:"name"`
		Phone     string                `json:"phone,omitempty"`
		Email     string                `json:"email,omitempty"`
		Address   string                `json:"address,omitempty"`
		Notes     string                `json:"notes,omitempty"`
		Status    models.CustomerStatus `json:"status"`
		Version   int                   `json:"version"`
	}{customer.Reference, customer.Name, customer.Phone, customer.Email, customer.Address,
		customer.Notes, customer.Status, customer.Version})
	return string(payload), err
}
