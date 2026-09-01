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
		SELECT id, username, COALESCE(email, ''), password_hash, role, status, created_at, updated_at
		FROM users WHERE username = ? COLLATE NOCASE
	`, username).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrUserNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("find user by username: %w", err)
	}
	return user, nil
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
