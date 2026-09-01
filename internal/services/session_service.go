package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"time"
)

var (
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenReuse   = errors.New("refresh token reuse detected")
)

type SessionResult struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	RefreshExpiresAt time.Time
	User            models.User
}

type SessionService struct {
	repository *repositories.SessionRepository
	tokens     *auth.TokenManager
	refreshTTL time.Duration
	now        func() time.Time
}

func NewSessionService(repository *repositories.SessionRepository, tokens *auth.TokenManager, refreshTTL time.Duration) (*SessionService, error) {
	if refreshTTL < time.Minute || refreshTTL > 30*24*time.Hour {
		return nil, errors.New("refresh token TTL must be between 1 minute and 30 days")
	}
	return &SessionService{repository: repository, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}, nil
}

func (s *SessionService) Start(ctx context.Context, user models.User, ipAddress, userAgent string) (SessionResult, error) {
	now := s.now().UTC()
	raw, hash, err := auth.NewRefreshToken()
	if err != nil {
		return SessionResult{}, err
	}
	sessionID, err := identity.NewID()
	if err != nil {
		return SessionResult{}, err
	}
	expiresAt := now.Add(s.refreshTTL)
	session := models.RefreshSession{
		ID: sessionID, UserID: user.ID, TokenHash: hash, TokenFamily: sessionID,
		ExpiresAt: expiresAt.Format(time.RFC3339Nano), IPAddress: ipAddress,
		UserAgent: userAgent, CreatedAt: now.Format(time.RFC3339Nano),
	}
	if err = s.repository.Create(ctx, session); err != nil {
		return SessionResult{}, err
	}
	accessToken, accessExpiresAt, err := s.tokens.Issue(user)
	if err != nil {
		return SessionResult{}, err
	}
	return SessionResult{AccessToken: accessToken, AccessExpiresAt: accessExpiresAt,
		RefreshToken: raw, RefreshExpiresAt: expiresAt, User: user}, nil
}

func (s *SessionService) Refresh(ctx context.Context, rawToken, ipAddress, userAgent string) (SessionResult, error) {
	if rawToken == "" {
		return SessionResult{}, ErrInvalidRefreshToken
	}
	now := s.now().UTC()
	newRaw, newHash, err := auth.NewRefreshToken()
	if err != nil {
		return SessionResult{}, err
	}
	newID, err := identity.NewID()
	if err != nil {
		return SessionResult{}, err
	}
	auditID, err := identity.NewID()
	if err != nil {
		return SessionResult{}, err
	}
	refreshExpiresAt := now.Add(s.refreshTTL)
	replacement := models.RefreshSession{
		ID: newID, TokenHash: newHash, ExpiresAt: refreshExpiresAt.Format(time.RFC3339Nano),
		IPAddress: ipAddress, UserAgent: userAgent, CreatedAt: now.Format(time.RFC3339Nano),
	}
	user, err := s.repository.Rotate(ctx, auth.HashRefreshToken(rawToken), replacement,
		auditID, now.Format(time.RFC3339Nano))
	if errors.Is(err, repositories.ErrRefreshTokenReused) {
		return SessionResult{}, ErrRefreshTokenReuse
	}
	if errors.Is(err, repositories.ErrRefreshSessionNotFound) {
		return SessionResult{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return SessionResult{}, err
	}
	accessToken, accessExpiresAt, err := s.tokens.Issue(user)
	if err != nil {
		return SessionResult{}, err
	}
	return SessionResult{AccessToken: accessToken, AccessExpiresAt: accessExpiresAt,
		RefreshToken: newRaw, RefreshExpiresAt: refreshExpiresAt, User: user}, nil
}

func (s *SessionService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrInvalidRefreshToken
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	err = s.repository.RevokeFamily(ctx, auth.HashRefreshToken(rawToken), auditID,
		s.now().UTC().Format(time.RFC3339Nano))
	if errors.Is(err, repositories.ErrRefreshSessionNotFound) {
		return ErrInvalidRefreshToken
	}
	return err
}
