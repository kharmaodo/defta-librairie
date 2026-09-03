package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidSale = errors.New("invalid sale data")

type SaleService struct {
	repository *repositories.SaleRepository
	now        func() time.Time
}

func NewSaleService(repository *repositories.SaleRepository) *SaleService {
	return &SaleService{repository: repository, now: time.Now}
}

func (s *SaleService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string,
	filter models.SaleFilter, offset, limit int) ([]models.Sale, int, error) {
	filter.Status = models.SaleStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))))
	if filter.Status != "" && filter.Status != models.SaleStatusDraft &&
		filter.Status != models.SaleStatusConfirmed && filter.Status != models.SaleStatusCancelled {
		return nil, 0, ErrInvalidSale
	}
	if !validSaleDate(filter.From) || !validSaleDate(filter.To) ||
		(filter.From != "" && filter.To != "" && filter.From > filter.To) {
		return nil, 0, ErrInvalidSale
	}
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil {
		return nil, 0, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 30
	}
	return s.repository.List(ctx, libraryID, filter, offset, limit)
}

func validSaleDate(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func (s *SaleService) Find(ctx context.Context, claims *auth.Claims, id string) (models.Sale, error) {
	if strings.TrimSpace(id) == "" {
		return models.Sale{}, ErrInvalidSale
	}
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.Sale{}, err
	}
	return s.repository.Find(ctx, id, libraryID)
}

func (s *SaleService) Create(ctx context.Context, claims *auth.Claims, input models.SaleInput) (models.Sale, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil {
		return models.Sale{}, err
	}
	if err = validateSaleInput(input, false); err != nil {
		return models.Sale{}, err
	}
	saleID, lineIDs, auditID, err := saleIDs(len(input.Lines))
	if err != nil {
		return models.Sale{}, err
	}
	now := s.now().UTC()
	reference := fmt.Sprintf("V-%s-%s", now.Format("20060102"), strings.ToUpper(strings.ReplaceAll(saleID, "-", "")[:8]))
	sale := models.Sale{ID: saleID, LibraryID: libraryID, Reference: reference,
		CustomerName: strings.TrimSpace(input.CustomerName), CreatedBy: claims.Subject}
	return s.repository.Create(ctx, sale, input.Lines, lineIDs, auditID, now.Format(time.RFC3339Nano))
}

func (s *SaleService) Update(ctx context.Context, claims *auth.Claims, id string, input models.SaleInput) (models.Sale, error) {
	if strings.TrimSpace(id) == "" {
		return models.Sale{}, ErrInvalidSale
	}
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.Sale{}, err
	}
	if err = validateSaleInput(input, true); err != nil {
		return models.Sale{}, err
	}
	_, lineIDs, auditID, err := saleIDs(len(input.Lines))
	if err != nil {
		return models.Sale{}, err
	}
	return s.repository.Update(ctx, id, libraryID, strings.TrimSpace(input.CustomerName), input.Lines,
		lineIDs, input.Version, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *SaleService) Confirm(ctx context.Context, claims *auth.Claims, id string, version int) (models.Sale, error) {
	return s.transition(ctx, claims, id, version, models.SaleStatusConfirmed)
}

func (s *SaleService) Cancel(ctx context.Context, claims *auth.Claims, id string, version int) (models.Sale, error) {
	return s.transition(ctx, claims, id, version, models.SaleStatusCancelled)
}

func (s *SaleService) transition(ctx context.Context, claims *auth.Claims, id string, version int,
	target models.SaleStatus) (models.Sale, error) {
	if strings.TrimSpace(id) == "" || version < 1 {
		return models.Sale{}, ErrInvalidSale
	}
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.Sale{}, err
	}
	sale, err := s.repository.Find(ctx, id, libraryID)
	if err != nil {
		return models.Sale{}, err
	}
	movementIDs := make([]string, len(sale.Lines))
	inventoryAuditIDs := make([]string, len(sale.Lines))
	for index := range sale.Lines {
		if movementIDs[index], err = identity.NewID(); err != nil {
			return models.Sale{}, err
		}
		if inventoryAuditIDs[index], err = identity.NewID(); err != nil {
			return models.Sale{}, err
		}
	}
	saleAuditID, err := identity.NewID()
	if err != nil {
		return models.Sale{}, err
	}
	return s.repository.Transition(ctx, id, libraryID, claims.Subject, version, target,
		movementIDs, inventoryAuditIDs, saleAuditID, s.now().UTC().Format(time.RFC3339Nano))
}

func validateSaleInput(input models.SaleInput, update bool) error {
	if len([]rune(strings.TrimSpace(input.CustomerName))) > 200 || len(input.Lines) < 1 || len(input.Lines) > 100 ||
		(update && input.Version < 1) {
		return ErrInvalidSale
	}
	seen := make(map[int64]bool, len(input.Lines))
	for _, line := range input.Lines {
		if line.BookID < 1 || line.Quantity < 1 || line.Quantity > 100000 || seen[line.BookID] {
			return ErrInvalidSale
		}
		seen[line.BookID] = true
	}
	return nil
}

func saleIDs(lines int) (string, []string, string, error) {
	saleID, err := identity.NewID()
	if err != nil {
		return "", nil, "", err
	}
	lineIDs := make([]string, lines)
	for index := range lineIDs {
		if lineIDs[index], err = identity.NewID(); err != nil {
			return "", nil, "", err
		}
	}
	auditID, err := identity.NewID()
	return saleID, lineIDs, auditID, err
}
