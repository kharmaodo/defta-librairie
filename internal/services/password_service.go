package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"errors"
	"time"
)

var (
	ErrInvalidCurrentPassword = errors.New("invalid current password")
	ErrPasswordUnchanged      = errors.New("new password must be different")
)

type passwordUserStore interface {
	FindByID(context.Context, string) (models.User, error)
	ChangePassword(context.Context, string, string, string, string, string) error
}

type PasswordService struct {
	users passwordUserStore
	now   func() time.Time
}

func NewPasswordService(users passwordUserStore) *PasswordService {
	return &PasswordService{users: users, now: time.Now}
}

func (s *PasswordService) Change(ctx context.Context, userID, currentPassword, newPassword, ipAddress string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	valid, err := auth.VerifyPassword(currentPassword, user.PasswordHash)
	if err != nil || !valid {
		return ErrInvalidCurrentPassword
	}
	unchanged, err := auth.VerifyPassword(newPassword, user.PasswordHash)
	if err == nil && unchanged {
		return ErrPasswordUnchanged
	}
	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.users.ChangePassword(ctx, userID, passwordHash, auditID, ipAddress,
		s.now().UTC().Format(time.RFC3339Nano))
}
