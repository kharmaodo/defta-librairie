package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrOwnerNotFound = errors.New("owner not found")
	ErrOwnerConflict = errors.New("owner username or email already exists")
	ErrOwnerNotLocked = errors.New("owner is not locked")
)

type OwnerRepository struct{ db *sql.DB }

func NewOwnerRepository(db *sql.DB) *OwnerRepository { return &OwnerRepository{db: db} }

func (r *OwnerRepository) Create(ctx context.Context, owner models.OwnerAccount, passwordHash, actorID, auditID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner creation: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users(id, username, email, password_hash, role, status, password_changed_at, created_at, updated_at)
		VALUES (?, ?, NULLIF(?, ''), ?, 'OWNER_LIBRARY', ?, ?, ?, ?)
	`, owner.ID, owner.Username, owner.Email, passwordHash, owner.Status,
		owner.CreatedAt, owner.CreatedAt, owner.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrOwnerConflict
	}
	if err != nil {
		return fmt.Errorf("insert owner: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO libraries(id, name, description, owner_user_id, status, created_at, updated_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`, owner.Library.ID, owner.Library.Name, owner.Library.Description, owner.ID,
		owner.Library.Status, owner.Library.CreatedAt, owner.Library.UpdatedAt); err != nil {
		return fmt.Errorf("insert owner library: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'CREATE_LIBRARY_OWNER', 'USER', ?, '{"role":"OWNER_LIBRARY"}', 1, ?)
	`, auditID, actorID, owner.ID, owner.CreatedAt); err != nil {
		return fmt.Errorf("audit owner creation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit owner creation: %w", err)
	}
	return nil
}

func (r *OwnerRepository) List(ctx context.Context) ([]models.OwnerAccount, error) {
	rows, err := r.db.QueryContext(ctx, ownerSelect+` ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list owners: %w", err)
	}
	defer rows.Close()
	owners := make([]models.OwnerAccount, 0)
	for rows.Next() {
		owner, scanErr := scanOwner(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan owner: %w", scanErr)
		}
		owners = append(owners, owner)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate owners: %w", err)
	}
	return owners, nil
}

func (r *OwnerRepository) FindByID(ctx context.Context, id string) (models.OwnerAccount, error) {
	owner, err := scanOwner(r.db.QueryRowContext(ctx, ownerSelect+` AND u.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return models.OwnerAccount{}, ErrOwnerNotFound
	}
	if err != nil {
		return models.OwnerAccount{}, fmt.Errorf("find owner: %w", err)
	}
	return owner, nil
}

func (r *OwnerRepository) Update(ctx context.Context, owner models.OwnerAccount, passwordHash, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner update: %w", err)
	}
	defer tx.Rollback()

	query := `UPDATE users SET username=?, email=NULLIF(?, ''), status=?, updated_at=?`
	args := []interface{}{owner.Username, owner.Email, owner.Status, now}
	if passwordHash != "" {
		query += `, password_hash=?, password_changed_at=?`
		args = append(args, passwordHash, now)
	}
	query += ` WHERE id=? AND role='OWNER_LIBRARY'`
	args = append(args, owner.ID)
	result, err := tx.ExecContext(ctx, query, args...)
	if isUniqueViolation(err) {
		return ErrOwnerConflict
	}
	if err != nil {
		return fmt.Errorf("update owner: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrOwnerNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE libraries SET name=?, description=NULLIF(?, ''), status=?, updated_at=?
		WHERE id=? AND owner_user_id=?
	`, owner.Library.Name, owner.Library.Description, owner.Library.Status, now,
		owner.Library.ID, owner.ID); err != nil {
		return fmt.Errorf("update owner library: %w", err)
	}
	if owner.Status != models.UserStatusActive || passwordHash != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE refresh_sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, now, owner.ID); err != nil {
			return fmt.Errorf("revoke owner sessions: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'UPDATE_LIBRARY_OWNER', 'USER', ?, '{"updated":true}', 1, ?)
	`, auditID, actorID, owner.ID, now); err != nil {
		return fmt.Errorf("audit owner update: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit owner update: %w", err)
	}
	return nil
}

func (r *OwnerRepository) Disable(ctx context.Context, ownerID, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner disable: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE users SET status='DISABLED', locked_until=NULL, updated_at=?
		WHERE id=? AND role='OWNER_LIBRARY'
	`, now, ownerID)
	if err != nil {
		return fmt.Errorf("disable owner: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrOwnerNotFound
	}
	if _, err = tx.ExecContext(ctx, `UPDATE libraries SET status='DISABLED', updated_at=? WHERE owner_user_id=?`, now, ownerID); err != nil {
		return fmt.Errorf("disable owner library: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE refresh_sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, now, ownerID); err != nil {
		return fmt.Errorf("revoke owner sessions: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'DISABLE_LIBRARY_OWNER', 'USER', ?, '{"status":"DISABLED"}', 1, ?)
	`, auditID, actorID, ownerID, now); err != nil {
		return fmt.Errorf("audit owner disable: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit owner disable: %w", err)
	}
	return nil
}

func (r *OwnerRepository) Unlock(ctx context.Context, ownerID, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin owner unlock: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE users SET status='ACTIVE', failed_login_attempts=0, locked_until=NULL, updated_at=?
		WHERE id=? AND role='OWNER_LIBRARY' AND status='LOCKED'
	`, now, ownerID)
	if err != nil {
		return fmt.Errorf("unlock owner: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("unlock owner rows: %w", err)
	}
	if rows != 1 {
		var exists int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=? AND role='OWNER_LIBRARY'`, ownerID).Scan(&exists); err != nil {
			return fmt.Errorf("check owner before unlock: %w", err)
		}
		if exists == 0 {
			return ErrOwnerNotFound
		}
		return ErrOwnerNotLocked
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE refresh_sessions SET revoked_at=COALESCE(revoked_at, ?) WHERE user_id=?
	`, now, ownerID); err != nil {
		return fmt.Errorf("revoke unlocked owner sessions: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'UNLOCK_LIBRARY_OWNER', 'USER', ?, '{"status":"ACTIVE","failed_login_attempts":0}', 1, ?)
	`, auditID, actorID, ownerID, now); err != nil {
		return fmt.Errorf("audit owner unlock: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit owner unlock: %w", err)
	}
	return nil
}

const ownerSelect = `
	SELECT u.id, u.username, COALESCE(u.email, ''), u.status, u.created_at, u.updated_at,
	       l.id, l.name, COALESCE(l.description, ''), l.status, l.created_at, l.updated_at
	FROM users u JOIN libraries l ON l.owner_user_id = u.id
	WHERE u.role = 'OWNER_LIBRARY'`

type rowScanner interface{ Scan(...interface{}) error }

func scanOwner(row rowScanner) (models.OwnerAccount, error) {
	var owner models.OwnerAccount
	err := row.Scan(&owner.ID, &owner.Username, &owner.Email, &owner.Status,
		&owner.CreatedAt, &owner.UpdatedAt, &owner.Library.ID, &owner.Library.Name,
		&owner.Library.Description, &owner.Library.Status, &owner.Library.CreatedAt,
		&owner.Library.UpdatedAt)
	return owner, err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
