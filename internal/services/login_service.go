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

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

const lockDuration = 15 * time.Minute

type loginUserStore interface {
	FindByUsername(context.Context, string) (models.User, error)
	RecordFailedLogin(context.Context, string, string, string, string, string) error
	RecordSuccessfulLogin(context.Context, string, string, string, string) error
	RecordUnknownLogin(context.Context, string, string, string) error
}

type LoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
	User        models.User
}

type LoginService struct {
	users     loginUserStore
	tokens    *auth.TokenManager
	dummyHash string
	now       func() time.Time
}

func NewLoginService(users loginUserStore, tokens *auth.TokenManager) (*LoginService, error) {
	dummyHash, err := auth.HashPassword("Dummy-Password-For-Timing-Only")
	if err != nil { return nil, err }
	return &LoginService{users: users, tokens: tokens, dummyHash: dummyHash, now: time.Now}, nil
}

func (s *LoginService) Login(ctx context.Context, username, password, ipAddress string) (LoginResult, error) {
	username = strings.TrimSpace(username)
	user, err := s.users.FindByUsername(ctx, username)
	if errors.Is(err, repositories.ErrUserNotFound) {
		_, _ = auth.VerifyPassword(password, s.dummyHash)
		s.auditUnknown(ctx, ipAddress)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil { return LoginResult{}, err }
	if user.Status == models.UserStatusDisabled {
		_, _ = auth.VerifyPassword(password, s.dummyHash)
		return LoginResult{}, ErrAccountDisabled
	}

	now := s.now().UTC()
	if user.Status == models.UserStatusLocked {
		lockedUntil, parseErr := time.Parse(time.RFC3339Nano, user.LockedUntil)
		if parseErr != nil || now.Before(lockedUntil) {
			_, _ = auth.VerifyPassword(password, s.dummyHash)
			return LoginResult{}, ErrAccountLocked
		}
	}

	valid, err := auth.VerifyPassword(password, user.PasswordHash)
	if err != nil { return LoginResult{}, ErrInvalidCredentials }
	if !valid {
		auditID, idErr := identity.NewID()
		if idErr != nil { return LoginResult{}, idErr }
		stamp := now.Format(time.RFC3339Nano)
		if err = s.users.RecordFailedLogin(ctx, user.ID, auditID, ipAddress, stamp, now.Add(lockDuration).Format(time.RFC3339Nano)); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{}, ErrInvalidCredentials
	}

	auditID, err := identity.NewID()
	if err != nil { return LoginResult{}, err }
	if err = s.users.RecordSuccessfulLogin(ctx, user.ID, auditID, ipAddress, now.Format(time.RFC3339Nano)); err != nil {
		return LoginResult{}, err
	}
	raw, expiresAt, err := s.tokens.Issue(user)
	if err != nil { return LoginResult{}, err }
	user.PasswordHash = ""
	return LoginResult{AccessToken: raw, ExpiresAt: expiresAt, User: user}, nil
}

func (s *LoginService) auditUnknown(ctx context.Context, ipAddress string) {
	auditID, err := identity.NewID()
	if err != nil { return }
	_ = s.users.RecordUnknownLogin(ctx, auditID, ipAddress, s.now().UTC().Format(time.RFC3339Nano))
}
