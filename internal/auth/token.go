package auth

import (
	"context"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid access token")

type Claims struct {
	Role                   models.UserRole `json:"role"`
	LibraryID              string          `json:"library_id,omitempty"`
	SessionID              string          `json:"sid,omitempty"`
	PasswordChangeRequired bool            `json:"password_change_required,omitempty"`
	jwt.RegisteredClaims
}

type claimsContextKey struct{}

type TokenManager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

func NewTokenManager(secret, issuer, audience string, ttl time.Duration) (*TokenManager, error) {
	if len([]byte(secret)) < 32 {
		return nil, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	if issuer == "" || audience == "" {
		return nil, errors.New("JWT issuer and audience are required")
	}
	if ttl <= 0 || ttl > 24*time.Hour {
		return nil, errors.New("JWT access TTL must be between 1 second and 24 hours")
	}
	return &TokenManager{secret: []byte(secret), issuer: issuer, audience: audience, ttl: ttl, now: time.Now}, nil
}

func (m *TokenManager) Issue(user models.User) (string, time.Time, error) {
	return m.issue(user, "")
}

func (m *TokenManager) IssueForSession(user models.User, sessionID string) (string, time.Time, error) {
	if sessionID == "" {
		return "", time.Time{}, ErrInvalidToken
	}
	return m.issue(user, sessionID)
}

func (m *TokenManager) issue(user models.User, sessionID string) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(m.ttl)
	jti, err := identity.NewID()
	if err != nil {
		return "", time.Time{}, err
	}
	claims := Claims{
		Role: user.Role, LibraryID: user.LibraryID, SessionID: sessionID,
		PasswordChangeRequired: user.MustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: m.issuer, Subject: user.ID,
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now), ID: jti,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return token, expiresAt, nil
}

func (m *TokenManager) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience), jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(), jwt.WithStrictDecoding(), jwt.WithTimeFunc(m.now))
	if err != nil || !token.Valid || claims.Subject == "" || claims.ID == "" || !validRole(claims.Role) {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func validRole(role models.UserRole) bool {
	return role == models.RoleSuperAdminRoot || role == models.RoleOwnerLibrary
}

func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return claims, ok
}
