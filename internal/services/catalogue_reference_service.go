package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
)

var ErrInvalidCatalogueReference = errors.New("invalid catalogue reference")

type CatalogueReferenceService struct {
	repository *repositories.CatalogueReferenceRepository
}

func NewCatalogueReferenceService(repository *repositories.CatalogueReferenceRepository) *CatalogueReferenceService {
	return &CatalogueReferenceService{repository: repository}
}

// List exposes the shared catalogue references to every authenticated manager.
// They are global data and deliberately do not use a library scope.
func (s *CatalogueReferenceService) List(ctx context.Context, claims *auth.Claims, kind string) ([]models.CatalogueReference, error) {
	if s == nil || s.repository == nil || claims == nil {
		return nil, ErrInvalidCatalogueReference
	}
	if kind != "category" && kind != "publisher" {
		return nil, ErrInvalidCatalogueReference
	}
	return s.repository.List(ctx, kind)
}
