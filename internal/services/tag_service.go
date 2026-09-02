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

var ErrInvalidTag = errors.New("invalid tag data")

type TagService struct {
	repository *repositories.TagRepository
	now        func() time.Time
}

func NewTagService(repository *repositories.TagRepository) *TagService {
	return &TagService{repository: repository, now: time.Now}
}

func (s *TagService) List(ctx context.Context, claims *auth.Claims, requestedLibrary string) ([]models.LibraryTag, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), false)
	if err != nil {
		return nil, err
	}
	if libraryID != "" {
		if err = s.ensureActive(ctx, libraryID); err != nil {
			return nil, err
		}
	}
	return s.repository.List(ctx, libraryID)
}

func (s *TagService) Create(ctx context.Context, claims *auth.Claims, name, requestedLibrary string) (models.LibraryTag, error) {
	libraryID, err := resolveBookScope(claims, strings.TrimSpace(requestedLibrary), true)
	if err != nil {
		return models.LibraryTag{}, err
	}
	name, normalized, err := normalizeTag(name)
	if err != nil {
		return models.LibraryTag{}, err
	}
	if err = s.ensureActive(ctx, libraryID); err != nil {
		return models.LibraryTag{}, err
	}
	id, err := identity.NewID()
	if err != nil {
		return models.LibraryTag{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.LibraryTag{}, err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	tag := models.LibraryTag{ID: id, LibraryID: libraryID, Name: name, CreatedAt: now, UpdatedAt: now}
	if err = s.repository.Create(ctx, tag, normalized, claims.Subject, auditID); err != nil {
		return models.LibraryTag{}, err
	}
	return tag, nil
}

func (s *TagService) Update(ctx context.Context, claims *auth.Claims, id, name string) (models.LibraryTag, error) {
	tag, err := s.authorizedTag(ctx, claims, id)
	if err != nil {
		return models.LibraryTag{}, err
	}
	name, normalized, err := normalizeTag(name)
	if err != nil {
		return models.LibraryTag{}, err
	}
	oldName := tag.Name
	tag.Name = name
	tag.UpdatedAt = s.now().UTC().Format(time.RFC3339Nano)
	auditID, err := identity.NewID()
	if err != nil {
		return models.LibraryTag{}, err
	}
	if err = s.repository.Update(ctx, tag, normalized, oldName, claims.Subject, auditID); err != nil {
		return models.LibraryTag{}, err
	}
	return tag, nil
}

func (s *TagService) Delete(ctx context.Context, claims *auth.Claims, id string) error {
	tag, err := s.authorizedTag(ctx, claims, id)
	if err != nil {
		return err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.Delete(ctx, tag, claims.Subject, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *TagService) authorizedTag(ctx context.Context, claims *auth.Claims, id string) (models.LibraryTag, error) {
	if strings.TrimSpace(id) == "" {
		return models.LibraryTag{}, ErrInvalidTag
	}
	tag, err := s.repository.Find(ctx, id)
	if err != nil {
		return models.LibraryTag{}, err
	}
	libraryID, err := resolveBookScope(claims, "", false)
	if err != nil {
		return models.LibraryTag{}, err
	}
	if libraryID != "" && libraryID != tag.LibraryID {
		return models.LibraryTag{}, repositories.ErrTagNotFound
	}
	if err = s.ensureActive(ctx, tag.LibraryID); err != nil {
		return models.LibraryTag{}, err
	}
	return tag, nil
}

func (s *TagService) ensureActive(ctx context.Context, libraryID string) error {
	active, err := s.repository.LibraryActive(ctx, libraryID)
	if err != nil {
		return err
	}
	if !active {
		return repositories.ErrLibraryUnavailable
	}
	return nil
}

func normalizeTag(value string) (string, string, error) {
	name := strings.Join(strings.Fields(value), " ")
	if len([]rune(name)) < 2 || len([]rune(name)) > 50 || strings.Contains(name, ",") {
		return "", "", ErrInvalidTag
	}
	return name, strings.ToLower(name), nil
}
