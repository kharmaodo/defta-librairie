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
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenReuse   = errors.New("refresh token reuse detected")
	ErrSessionNotFound     = errors.New("active session not found")
	ErrInvalidSessionFilter = errors.New("invalid session filter")
)

type SessionResult struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             models.User
}

func (s *SessionService) ListActive(ctx context.Context, claims *auth.Claims, filter models.SessionFilter, offset, limit int) ([]models.ActiveSession, int, error) {
	if claims == nil || claims.Role != models.RoleSuperAdminRoot && claims.Role != models.RoleOwnerLibrary {
		return nil, 0, ErrSessionNotFound
	}
	filter.Username = strings.TrimSpace(filter.Username)
	filter.Role = strings.ToUpper(strings.TrimSpace(filter.Role))
	filter.IPAddress = strings.TrimSpace(filter.IPAddress)
	filter.UserAgent = strings.TrimSpace(filter.UserAgent)
	if len([]rune(filter.Username)) > 64 || len([]rune(filter.IPAddress)) > 64 || len([]rune(filter.UserAgent)) > 200 ||
		filter.Role != "" && filter.Role != string(models.RoleSuperAdminRoot) && filter.Role != string(models.RoleOwnerLibrary) {
		return nil, 0, ErrInvalidSessionFilter
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
	scopedUserID := ""
	if claims.Role == models.RoleOwnerLibrary {
		scopedUserID = claims.Subject
		if filter.Username != "" || filter.Role != "" {
			return nil, 0, ErrInvalidSessionFilter
		}
	}
	return s.repository.ListActive(ctx, scopedUserID, s.now().UTC().Format(time.RFC3339Nano), filter, offset, limit)
}

func (s *SessionService) RevokeActive(ctx context.Context, claims *auth.Claims, sessionID string) error {
	if claims == nil || sessionID == "" {
		return ErrSessionNotFound
	}
	scopedUserID := ""
	if claims.Role == models.RoleOwnerLibrary {
		scopedUserID = claims.Subject
	} else if claims.Role != models.RoleSuperAdminRoot {
		return ErrSessionNotFound
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	err = s.repository.RevokeActive(ctx, sessionID, scopedUserID, claims.Subject, auditID,
		s.now().UTC().Format(time.RFC3339Nano))
	if errors.Is(err, repositories.ErrActiveSessionNotFound) {
		return ErrSessionNotFound
	}
	return err
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
	accessToken, accessExpiresAt, err := s.tokens.IssueForSession(user, sessionID)
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
	accessToken, accessExpiresAt, err := s.tokens.IssueForSession(user, newID)
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
