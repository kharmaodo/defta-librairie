package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
	"time"
)

var (
	ErrRefreshSessionNotFound = errors.New("refresh session not found")
	ErrRefreshTokenReused     = errors.New("refresh token reuse detected")
)

type SessionRepository struct{ db *sql.DB }

func NewSessionRepository(db *sql.DB) *SessionRepository { return &SessionRepository{db: db} }

func (r *SessionRepository) IsActive(ctx context.Context, sessionID, userID string,
	role models.UserRole, libraryID string, now time.Time) (bool, error) {
	var expiresAt, userStatus, storedRole, storedLibraryID, libraryStatus string
	err := r.db.QueryRowContext(ctx, `
		SELECT s.expires_at, u.status, u.role, COALESCE(l.id, ''), COALESCE(l.status, '')
		FROM refresh_sessions s
		JOIN users u ON u.id=s.user_id
		LEFT JOIN libraries l ON l.owner_user_id=u.id
		WHERE s.id=? AND s.user_id=? AND s.revoked_at IS NULL
	`, sessionID, userID).Scan(&expiresAt, &userStatus, &storedRole, &storedLibraryID, &libraryStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("validate access session: %w", err)
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || !expiry.After(now.UTC()) || userStatus != string(models.UserStatusActive) ||
		storedRole != string(role) {
		return false, nil
	}
	if role == models.RoleOwnerLibrary {
		return libraryID != "" && storedLibraryID == libraryID && libraryStatus == "ACTIVE", nil
	}
	return role == models.RoleSuperAdminRoot && libraryID == "", nil
}

func (r *SessionRepository) Create(ctx context.Context, session models.RefreshSession) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_sessions(id, user_id, token_hash, token_family, expires_at,
		                             ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
	`, session.ID, session.UserID, session.TokenHash, session.TokenFamily,
		session.ExpiresAt, session.IPAddress, session.UserAgent, session.CreatedAt)
	if err != nil {
		return fmt.Errorf("create refresh session: %w", err)
	}
	return nil
}

func (r *SessionRepository) Rotate(ctx context.Context, oldTokenHash string, replacement models.RefreshSession, auditID, now string) (models.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.User{}, fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer tx.Rollback()

	var session models.RefreshSession
	var user models.User
	var libraryStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.token_family, s.expires_at, COALESCE(s.revoked_at, ''),
		       COALESCE(s.replaced_by_id, ''), u.username, COALESCE(u.email, ''), u.role,
		       u.status, COALESCE(l.id, ''), COALESCE(l.status, '')
		FROM refresh_sessions s
		JOIN users u ON u.id=s.user_id
		LEFT JOIN libraries l ON l.owner_user_id=u.id
		WHERE s.token_hash=?
	`, oldTokenHash).Scan(&session.ID, &session.UserID, &session.TokenFamily,
		&session.ExpiresAt, &session.RevokedAt, &session.ReplacedByID, &user.Username,
		&user.Email, &user.Role, &user.Status, &user.LibraryID, &libraryStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrRefreshSessionNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("find refresh session: %w", err)
	}
	user.ID = session.UserID

	if session.RevokedAt != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE refresh_sessions SET revoked_at=COALESCE(revoked_at, ?) WHERE token_family=?`, now, session.TokenFamily); err != nil {
			return models.User{}, fmt.Errorf("revoke reused token family: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
			VALUES (?, ?, 'REFRESH_TOKEN_REUSE', 'SESSION', ?, '{"family_revoked":true}', 0, ?)
		`, auditID, session.UserID, session.ID, now); err != nil {
			return models.User{}, fmt.Errorf("audit refresh reuse: %w", err)
		}
		if err = tx.Commit(); err != nil {
			return models.User{}, fmt.Errorf("commit refresh reuse: %w", err)
		}
		return models.User{}, ErrRefreshTokenReused
	}

	expiresAt, parseErr := time.Parse(time.RFC3339Nano, session.ExpiresAt)
	if parseErr != nil || !expiresAt.After(mustParseTime(now)) || user.Status != models.UserStatusActive ||
		user.Role == models.RoleOwnerLibrary && libraryStatus != "ACTIVE" {
		return models.User{}, ErrRefreshSessionNotFound
	}

	replacement.UserID = session.UserID
	replacement.TokenFamily = session.TokenFamily
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO refresh_sessions(id, user_id, token_hash, token_family, expires_at,
		                             ip_address, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
	`, replacement.ID, replacement.UserID, replacement.TokenHash, replacement.TokenFamily,
		replacement.ExpiresAt, replacement.IPAddress, replacement.UserAgent, replacement.CreatedAt); err != nil {
		return models.User{}, fmt.Errorf("insert rotated refresh session: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE refresh_sessions SET revoked_at=?, replaced_by_id=?, last_used_at=?
		WHERE id=? AND revoked_at IS NULL
	`, now, replacement.ID, now, session.ID)
	if err != nil {
		return models.User{}, fmt.Errorf("revoke rotated refresh session: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return models.User{}, ErrRefreshTokenReused
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'TOKEN_REFRESHED', 'SESSION', ?, '{"rotated":true}', 1, ?)
	`, auditID, session.UserID, replacement.ID, now); err != nil {
		return models.User{}, fmt.Errorf("audit refresh rotation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.User{}, fmt.Errorf("commit refresh rotation: %w", err)
	}
	return user, nil
}

func (r *SessionRepository) RevokeFamily(ctx context.Context, tokenHash, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin logout: %w", err)
	}
	defer tx.Rollback()
	var sessionID, userID, family string
	err = tx.QueryRowContext(ctx, `SELECT id, user_id, token_family FROM refresh_sessions WHERE token_hash=?`, tokenHash).
		Scan(&sessionID, &userID, &family)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRefreshSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("find logout session: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE refresh_sessions SET revoked_at=COALESCE(revoked_at, ?) WHERE token_family=?`, now, family); err != nil {
		return fmt.Errorf("revoke logout family: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'LOGOUT', 'SESSION', ?, '{"family_revoked":true}', 1, ?)
	`, auditID, userID, sessionID, now); err != nil {
		return fmt.Errorf("audit logout: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit logout: %w", err)
	}
	return nil
}

func mustParseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
