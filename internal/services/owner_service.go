package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var ErrInvalidOwner = errors.New("invalid owner data")

var ownerUsernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,64}$`)

type OwnerService struct {
	repository *repositories.OwnerRepository
	now        func() time.Time
}

func NewOwnerService(repository *repositories.OwnerRepository) *OwnerService {
	return &OwnerService{repository: repository, now: time.Now}
}

func (s *OwnerService) Create(ctx context.Context, input models.OwnerCreateInput, actorID string) (models.OwnerAccount, error) {
	normalizeCreateInput(&input)
	if err := validateOwner(input.Username, input.Email, input.LibraryName, input.LibraryDescription); err != nil {
		return models.OwnerAccount{}, err
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return models.OwnerAccount{}, err
	}
	ownerID, err := identity.NewID()
	if err != nil {
		return models.OwnerAccount{}, err
	}
	libraryID, err := identity.NewID()
	if err != nil {
		return models.OwnerAccount{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.OwnerAccount{}, err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	owner := models.OwnerAccount{
		ID: ownerID, Username: input.Username, Email: input.Email,
		Status: models.UserStatusActive, CreatedAt: now, UpdatedAt: now,
		Library: models.Library{
			ID: libraryID, Name: input.LibraryName, Description: input.LibraryDescription,
			Status: models.LibraryStatusActive, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err = s.repository.Create(ctx, owner, passwordHash, actorID, auditID); err != nil {
		return models.OwnerAccount{}, err
	}
	return owner, nil
}

func (s *OwnerService) List(ctx context.Context) ([]models.OwnerAccount, error) {
	return s.repository.List(ctx)
}

func (s *OwnerService) Search(ctx context.Context, query, userStatus, libraryStatus string, offset, limit int) ([]models.OwnerAccount, int, error) {
	query = strings.TrimSpace(query)
	userStatus = strings.ToUpper(strings.TrimSpace(userStatus))
	libraryStatus = strings.ToUpper(strings.TrimSpace(libraryStatus))
	if len([]rune(query)) > 200 || (userStatus != "" && !validOwnerSearchStatus(userStatus)) ||
		(libraryStatus != "" && !validLibrarySearchStatus(libraryStatus)) {
		return nil, 0, ErrInvalidOwner
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
	return s.repository.Search(ctx, query, userStatus, libraryStatus, offset, limit)
}

func (s *OwnerService) Find(ctx context.Context, id string) (models.OwnerAccount, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *OwnerService) Update(ctx context.Context, id string, input models.OwnerUpdateInput, actorID string) (models.OwnerAccount, error) {
	owner, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return models.OwnerAccount{}, err
	}
	applyOwnerUpdate(&owner, input)
	if owner.Status == models.UserStatusDisabled {
		owner.Library.Status = models.LibraryStatusDisabled
	}
	if err = validateOwner(owner.Username, owner.Email, owner.Library.Name, owner.Library.Description); err != nil {
		return models.OwnerAccount{}, err
	}
	if !validUserStatus(owner.Status) || !validLibraryStatus(owner.Library.Status) {
		return models.OwnerAccount{}, ErrInvalidOwner
	}
	passwordHash := ""
	if input.Password != nil {
		hashes, historyErr := s.repository.PasswordHashes(ctx, id, 4)
		if historyErr != nil {
			return models.OwnerAccount{}, historyErr
		}
		if passwordMatchesHistory(*input.Password, hashes) {
			return models.OwnerAccount{}, ErrPasswordReused
		}
		passwordHash, err = auth.HashPassword(*input.Password)
		if err != nil {
			return models.OwnerAccount{}, err
		}
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.OwnerAccount{}, err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	owner.UpdatedAt = now
	owner.Library.UpdatedAt = now
	if err = s.repository.Update(ctx, owner, passwordHash, actorID, auditID, now); err != nil {
		return models.OwnerAccount{}, err
	}
	return owner, nil
}

func (s *OwnerService) Disable(ctx context.Context, id, actorID string) error {
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.Disable(ctx, id, actorID, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *OwnerService) Unlock(ctx context.Context, id, actorID string) error {
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.Unlock(ctx, id, actorID, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *OwnerService) Reactivate(ctx context.Context, id, actorID string) error {
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.Reactivate(ctx, id, actorID, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *OwnerService) ResetPassword(ctx context.Context, id, password, actorID string) error {
	hashes, err := s.repository.PasswordHashes(ctx, id, 4)
	if err != nil {
		return err
	}
	if passwordMatchesHistory(password, hashes) {
		return ErrPasswordReused
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.ResetPassword(ctx, id, passwordHash, actorID, auditID,
		s.now().UTC().Format(time.RFC3339Nano))
}

func normalizeCreateInput(input *models.OwnerCreateInput) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.LibraryName = strings.TrimSpace(input.LibraryName)
	input.LibraryDescription = strings.TrimSpace(input.LibraryDescription)
}

func validateOwner(username, email, libraryName, libraryDescription string) error {
	if !ownerUsernamePattern.MatchString(username) || len([]rune(libraryName)) < 2 ||
		len([]rune(libraryName)) > 120 || len([]rune(libraryDescription)) > 1000 {
		return ErrInvalidOwner
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email {
			return ErrInvalidOwner
		}
	}
	return nil
}

func applyOwnerUpdate(owner *models.OwnerAccount, input models.OwnerUpdateInput) {
	if input.Username != nil {
		owner.Username = strings.TrimSpace(*input.Username)
	}
	if input.Email != nil {
		owner.Email = strings.TrimSpace(*input.Email)
	}
	if input.Status != nil {
		owner.Status = *input.Status
	}
	if input.LibraryName != nil {
		owner.Library.Name = strings.TrimSpace(*input.LibraryName)
	}
	if input.LibraryDescription != nil {
		owner.Library.Description = strings.TrimSpace(*input.LibraryDescription)
	}
	if input.LibraryStatus != nil {
		owner.Library.Status = *input.LibraryStatus
	}
}

func validUserStatus(status models.UserStatus) bool {
	return status == models.UserStatusActive || status == models.UserStatusDisabled
}

func validLibraryStatus(status models.LibraryStatus) bool {
	return status == models.LibraryStatusActive || status == models.LibraryStatusDisabled
}

func validOwnerSearchStatus(status string) bool {
	return status == string(models.UserStatusActive) || status == string(models.UserStatusDisabled) ||
		status == string(models.UserStatusLocked)
}

func validLibrarySearchStatus(status string) bool {
	return status == string(models.LibraryStatusActive) || status == string(models.LibraryStatusDisabled)
}
