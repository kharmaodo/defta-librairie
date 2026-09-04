package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrInvalidPurchase = errors.New("invalid purchase data")

type PurchaseService struct {
	repository *repositories.PurchaseRepository
	now        func() time.Time
}

func NewPurchaseService(repository *repositories.PurchaseRepository) *PurchaseService {
	return &PurchaseService{repository: repository, now: time.Now}
}

func (s *PurchaseService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string,
	filter models.PurchaseFilter, offset, limit int) ([]models.Purchase, int, error) {
	filter.Status = models.PurchaseStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))))
	filter.SupplierID = strings.TrimSpace(filter.SupplierID)
	if filter.Status != "" && filter.Status != models.PurchaseStatusDraft &&
		filter.Status != models.PurchaseStatusReceived && filter.Status != models.PurchaseStatusCancelled {
		return nil, 0, ErrInvalidPurchase
	}
	if !validSaleDate(filter.From) || !validSaleDate(filter.To) ||
		(filter.From != "" && filter.To != "" && filter.From > filter.To) {
		return nil, 0, ErrInvalidPurchase
	}
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil { return nil, 0, err }
	if offset < 0 { offset = 0 }
	if limit < 1 || limit > 100 { limit = 30 }
	return s.repository.List(ctx, libraryID, filter, offset, limit)
}

func (s *PurchaseService) Find(ctx context.Context, claims *auth.Claims, id string) (models.Purchase, error) {
	if strings.TrimSpace(id) == "" { return models.Purchase{}, ErrInvalidPurchase }
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return models.Purchase{}, err }
	return s.repository.Find(ctx, id, libraryID)
}

func (s *PurchaseService) Create(ctx context.Context, claims *auth.Claims, input models.PurchaseInput) (models.Purchase, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil { return models.Purchase{}, err }
	if err = validatePurchaseInput(input, false); err != nil { return models.Purchase{}, err }
	purchaseID, lineIDs, auditID, err := purchaseIDs(len(input.Lines))
	if err != nil { return models.Purchase{}, err }
	now := s.now().UTC()
	reference := fmt.Sprintf("A-%s-%s", now.Format("20060102"), strings.ToUpper(strings.ReplaceAll(purchaseID, "-", "")[:8]))
	purchase := models.Purchase{ID:purchaseID, LibraryID:libraryID, SupplierID:strings.TrimSpace(input.SupplierID),
		Reference:reference, CreatedBy:claims.Subject}
	return s.repository.Create(ctx, purchase, input.Lines, lineIDs, auditID, now.Format(time.RFC3339Nano))
}

func (s *PurchaseService) Update(ctx context.Context, claims *auth.Claims, id string,
	input models.PurchaseInput) (models.Purchase, error) {
	if strings.TrimSpace(id) == "" { return models.Purchase{}, ErrInvalidPurchase }
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return models.Purchase{}, err }
	if input.LibraryID != "" && strings.TrimSpace(input.LibraryID) != libraryID && libraryID != "" {
		return models.Purchase{}, ErrBookForbidden
	}
	if err = validatePurchaseInput(input, true); err != nil { return models.Purchase{}, err }
	_, lineIDs, auditID, err := purchaseIDs(len(input.Lines))
	if err != nil { return models.Purchase{}, err }
	return s.repository.Update(ctx, id, libraryID, strings.TrimSpace(input.SupplierID), input.Lines,
		lineIDs, input.Version, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *PurchaseService) Delete(ctx context.Context, claims *auth.Claims, id string, version int) error {
	if strings.TrimSpace(id) == "" || version < 1 { return ErrInvalidPurchase }
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil { return err }
	auditID, err := identity.NewID()
	if err != nil { return err }
	return s.repository.Delete(ctx, id, libraryID, version, claims.Subject, auditID,
		s.now().UTC().Format(time.RFC3339Nano))
}

func validatePurchaseInput(input models.PurchaseInput, update bool) error {
	if strings.TrimSpace(input.SupplierID) == "" || len(input.Lines) < 1 || len(input.Lines) > 100 ||
		(update && input.Version < 1) { return ErrInvalidPurchase }
	seen := make(map[int64]bool, len(input.Lines))
	for _, line := range input.Lines {
		if line.BookID < 1 || line.Quantity < 1 || line.Quantity > 100000 || line.UnitCost < 0 ||
			math.IsNaN(line.UnitCost) || math.IsInf(line.UnitCost, 0) || seen[line.BookID] {
			return ErrInvalidPurchase
		}
		seen[line.BookID] = true
	}
	return nil
}

func purchaseIDs(lines int) (string, []string, string, error) {
	purchaseID, err := identity.NewID()
	if err != nil { return "", nil, "", err }
	lineIDs := make([]string, lines)
	for index := range lineIDs {
		if lineIDs[index], err = identity.NewID(); err != nil { return "", nil, "", err }
	}
	auditID, err := identity.NewID()
	return purchaseID, lineIDs, auditID, err
}
