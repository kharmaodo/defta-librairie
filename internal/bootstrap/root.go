package bootstrap

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	ErrRootAlreadyExists   = errors.New("a SUPER_ADMIN_ROOT already exists")
	ErrMissingRootConfig   = errors.New("DEFTA_ROOT_USERNAME and DEFTA_ROOT_PASSWORD are required")
	ErrMissingResetPassword = errors.New("DEFTA_ROOT_NEW_PASSWORD is required")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,64}$`)

type RootInput struct {
	Username string
	Email    string
	Password string
}

func ResetRootPasswordFromEnvironment(ctx context.Context, db *sql.DB) (models.User, error) {
	password := os.Getenv("DEFTA_ROOT_NEW_PASSWORD")
	if password == "" {
		return models.User{}, ErrMissingResetPassword
	}

	repository := repositories.NewUserRepository(db)
	user, err := repository.FindByRole(ctx, models.RoleSuperAdminRoot)
	if err != nil {
		return models.User{}, err
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return models.User{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.User{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err = repository.ResetRootPassword(ctx, user.ID, passwordHash, auditID, now); err != nil {
		return models.User{}, err
	}
	user.PasswordHash = ""
	user.Status = models.UserStatusActive
	user.FailedLoginAttempts = 0
	user.LockedUntil = ""
	return user, nil
}

func RootFromEnvironment(ctx context.Context, db *sql.DB) (models.User, error) {
	return CreateRoot(ctx, db, RootInput{
		Username: os.Getenv("DEFTA_ROOT_USERNAME"),
		Email:    os.Getenv("DEFTA_ROOT_EMAIL"),
		Password: os.Getenv("DEFTA_ROOT_PASSWORD"),
	})
}

func CreateRoot(ctx context.Context, db *sql.DB, input RootInput) (models.User, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	if input.Username == "" || input.Password == "" {
		return models.User{}, ErrMissingRootConfig
	}
	if !usernamePattern.MatchString(input.Username) {
		return models.User{}, errors.New("username must contain 3 to 64 letters, digits, dots, dashes or underscores")
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || address.Address != input.Email {
			return models.User{}, errors.New("invalid root email")
		}
	}

	repository := repositories.NewUserRepository(db)
	count, err := repository.CountByRole(ctx, models.RoleSuperAdminRoot)
	if err != nil {
		return models.User{}, err
	}
	if count > 0 {
		return models.User{}, ErrRootAlreadyExists
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return models.User{}, err
	}
	userID, err := identity.NewID()
	if err != nil {
		return models.User{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return models.User{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	user := models.User{
		ID: userID, Username: input.Username, Email: input.Email,
		PasswordHash: passwordHash, Role: models.RoleSuperAdminRoot,
		Status: models.UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}

	if err = repository.CreateRoot(ctx, user, auditID); err != nil {
		return models.User{}, fmt.Errorf("create root account: %w", err)
	}
	return user, nil
}
