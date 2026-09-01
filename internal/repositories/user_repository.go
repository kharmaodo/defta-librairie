package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) CountByRole(ctx context.Context, role models.UserRole) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = ?", role).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return count, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (models.User, error) {
	var user models.User
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, COALESCE(u.email, ''), u.password_hash, u.role, u.status,
		       u.created_at, u.updated_at, u.failed_login_attempts, COALESCE(u.locked_until, ''),
		       COALESCE(l.id, '')
		FROM users u LEFT JOIN libraries l ON l.owner_user_id = u.id
		WHERE u.username = ? COLLATE NOCASE
	`, username).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
		&user.Status, &user.CreatedAt, &user.UpdatedAt, &user.FailedLoginAttempts,
		&user.LockedUntil, &user.LibraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("find user by username: %w", err)
	}
	return user, nil
}

func (r *UserRepository) RecordFailedLogin(ctx context.Context, userID, auditID, ipAddress, now, lockedUntil string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin failed login: %w", err) }
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET
			failed_login_attempts = failed_login_attempts + 1,
			status = CASE WHEN failed_login_attempts + 1 >= 5 THEN 'LOCKED' ELSE status END,
			locked_until = CASE WHEN failed_login_attempts + 1 >= 5 THEN ? ELSE locked_until END,
			updated_at = ?
		WHERE id = ?
	`, lockedUntil, now, userID); err != nil { return fmt.Errorf("update failed login: %w", err) }
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, ip_address, success, created_at)
		VALUES (?, ?, 'LOGIN_FAILED', 'SESSION', ?, '{"reason":"invalid_credentials"}', ?, 0, ?)
	`, auditID, userID, userID, ipAddress, now); err != nil { return fmt.Errorf("audit failed login: %w", err) }
	if err = tx.Commit(); err != nil { return fmt.Errorf("commit failed login: %w", err) }
	return nil
}

func (r *UserRepository) RecordSuccessfulLogin(ctx context.Context, userID, auditID, ipAddress, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin successful login: %w", err) }
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
		UPDATE users SET failed_login_attempts = 0, locked_until = NULL, status = 'ACTIVE',
		last_login_at = ?, updated_at = ? WHERE id = ?
	`, now, now, userID); err != nil { return fmt.Errorf("update successful login: %w", err) }
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, ip_address, success, created_at)
		VALUES (?, ?, 'LOGIN_SUCCEEDED', 'SESSION', ?, ?, 1, ?)
	`, auditID, userID, userID, ipAddress, now); err != nil { return fmt.Errorf("audit successful login: %w", err) }
	if err = tx.Commit(); err != nil { return fmt.Errorf("commit successful login: %w", err) }
	return nil
}

func (r *UserRepository) RecordUnknownLogin(ctx context.Context, auditID, ipAddress, now string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs(id, action, resource_type, new_values, ip_address, success, created_at)
		VALUES (?, 'LOGIN_FAILED', 'SESSION', '{"reason":"invalid_credentials"}', ?, 0, ?)
	`, auditID, ipAddress, now)
	if err != nil { return fmt.Errorf("audit unknown login: %w", err) }
	return nil
}

func (r *UserRepository) CreateRoot(ctx context.Context, user models.User, auditID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin root creation: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO users(id, username, email, password_hash, role, status, password_changed_at, created_at, updated_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.Status, user.CreatedAt, user.CreatedAt, user.UpdatedAt); err != nil {
		return fmt.Errorf("insert root user: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'BOOTSTRAP_SUPER_ADMIN', 'USER', ?, '{"role":"SUPER_ADMIN_ROOT"}', 1, ?)
	`, auditID, user.ID, user.ID, user.CreatedAt); err != nil {
		return fmt.Errorf("audit root creation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit root creation: %w", err)
	}
	return nil
}
