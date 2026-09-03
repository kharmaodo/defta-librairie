package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"strings"
	"time"
)

var ErrInvalidInventory = errors.New("invalid inventory data")

type InventoryService struct {
	repository *repositories.InventoryRepository
	now        func() time.Time
}

func NewInventoryService(repository *repositories.InventoryRepository) *InventoryService {
	return &InventoryService{repository: repository, now: time.Now}
}

func (s *InventoryService) Find(ctx context.Context, claims *auth.Claims, bookID int) (models.BookInventory, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.BookInventory{}, err
	}
	return s.repository.Find(ctx, bookID, libraryID)
}

func (s *InventoryService) Move(ctx context.Context, claims *auth.Claims, bookID int,
	movementType models.InventoryMovementType, quantity, version int, reason string) (models.BookInventory, error) {
	if claims == nil || bookID < 1 || quantity < 1 || version < 1 || len([]rune(strings.TrimSpace(reason))) > 500 ||
		(movementType != models.InventoryMovementEntry && movementType != models.InventoryMovementExit) {
		return models.BookInventory{}, ErrInvalidInventory
	}
	return s.apply(ctx, claims, bookID, movementType, quantity, version, reason)
}

func (s *InventoryService) Adjust(ctx context.Context, claims *auth.Claims, bookID, quantity, version int,
	reason string) (models.BookInventory, error) {
	if claims == nil || bookID < 1 || quantity < 0 || version < 1 || strings.TrimSpace(reason) == "" ||
		len([]rune(strings.TrimSpace(reason))) > 500 {
		return models.BookInventory{}, ErrInvalidInventory
	}
	return s.apply(ctx, claims, bookID, models.InventoryMovementAdjustment, quantity, version, reason)
}

func (s *InventoryService) apply(ctx context.Context, claims *auth.Claims, bookID int,
	movementType models.InventoryMovementType, quantity, version int, reason string) (models.BookInventory, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.BookInventory{}, err
	}
	movementID, err := identity.NewID()
	if err != nil {
		return models.BookInventory{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.BookInventory{}, err
	}
	return s.repository.ApplyMovement(ctx, bookID, libraryID, claims.Subject, movementID, auditID,
		movementType, quantity, version, strings.TrimSpace(reason), s.now().UTC().Format(time.RFC3339Nano))
}
