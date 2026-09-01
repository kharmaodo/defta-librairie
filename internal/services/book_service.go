package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"math"
	"strings"
	"time"
)

var (
	ErrInvalidBook   = errors.New("invalid book data")
	ErrBookForbidden = errors.New("book access forbidden")
)

type BookService struct {
	repository *repositories.BookRepository
	now        func() time.Time
}

func NewBookService(repository *repositories.BookRepository) *BookService {
	return &BookService{repository: repository, now: time.Now}
}

func (s *BookService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string, offset, limit int) ([]models.Book, int, error) {
	libraryID, err := resolveBookScope(claims, requestedLibrary, false)
	if err != nil {
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
	return s.repository.List(ctx, libraryID, offset, limit)
}

func (s *BookService) Find(ctx context.Context, claims *auth.Claims, id int) (models.Book, error) {
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.Book{}, err
	}
	return s.repository.Find(ctx, id, libraryID)
}

func (s *BookService) Create(ctx context.Context, claims *auth.Claims, input models.BookInput) (models.Book, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(input.LibraryID), true)
	if err != nil {
		return models.Book{}, err
	}
	input.LibraryID = libraryID
	normalizeBook(&input)
	if err = validateBook(input, false); err != nil {
		return models.Book{}, err
	}
	active, err := s.repository.LibraryActive(ctx, libraryID)
	if err != nil {
		return models.Book{}, err
	}
	if !active {
		return models.Book{}, repositories.ErrLibraryUnavailable
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.Book{}, err
	}
	return s.repository.Create(ctx, input, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *BookService) Update(ctx context.Context, claims *auth.Claims, id int, input models.BookInput) (models.Book, error) {
	scope, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.Book{}, err
	}
	existing, err := s.repository.Find(ctx, id, scope)
	if err != nil {
		return models.Book{}, err
	}
	if input.LibraryID != "" && input.LibraryID != existing.LibraryID {
		return models.Book{}, ErrBookForbidden
	}
	input.LibraryID = existing.LibraryID
	normalizeBook(&input)
	if err = validateBook(input, true); err != nil {
		return models.Book{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.Book{}, err
	}
	return s.repository.Update(ctx, id, input, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *BookService) Delete(ctx context.Context, claims *auth.Claims, id int) error {
	scope, err := resolveBookScope(claims, "", false)
	if err != nil {
		return err
	}
	book, err := s.repository.Find(ctx, id, scope)
	if err != nil {
		return err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.Delete(ctx, id, book.LibraryID, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func resolveBookScope(claims *auth.Claims, requestedLibrary string, required bool) (string, error) {
	if claims == nil {
		return "", ErrInvalidBook
	}
	switch claims.Role {
	case models.RoleSuperAdminRoot:
		if required && requestedLibrary == "" {
			return "", ErrInvalidBook
		}
		return requestedLibrary, nil
	case models.RoleOwnerLibrary:
		if claims.LibraryID == "" {
			return "", ErrBookForbidden
		}
		if requestedLibrary != "" && requestedLibrary != claims.LibraryID {
			return "", ErrBookForbidden
		}
		return claims.LibraryID, nil
	default:
		return "", ErrBookForbidden
	}
}

func normalizeBook(input *models.BookInput) {
	input.Title = strings.TrimSpace(input.Title)
	input.Auteur = strings.TrimSpace(input.Auteur)
	input.Editeur = strings.TrimSpace(input.Editeur)
	input.Status = strings.TrimSpace(input.Status)
	input.Tags = strings.TrimSpace(input.Tags)
	input.Categorie = strings.TrimSpace(input.Categorie)
	input.CoverURL = strings.TrimSpace(input.CoverURL)
}

func validateBook(input models.BookInput, update bool) error {
	if input.Title == "" || len([]rune(input.Title)) > 300 || input.Price < 0 ||
		math.IsNaN(input.Price) || math.IsInf(input.Price, 0) || input.Volume < 0 ||
		len([]rune(input.Tags)) > 500 || len([]rune(input.Status)) > 50 ||
		len([]rune(input.Categorie)) > 120 || len([]rune(input.CoverURL)) > 2048 {
		return ErrInvalidBook
	}
	if update && input.Version < 1 {
		return ErrInvalidBook
	}
	return nil
}
