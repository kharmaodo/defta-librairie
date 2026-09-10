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
	ErrCashRegisterNotFound = errors.New("cash register not found")
	ErrCashRegisterConflict = errors.New("cash register already exists")
	ErrCashRegisterVersion  = errors.New("cash register version conflict")
	ErrCashRegisterState    = errors.New("cash register state conflict")
)

type CashRegisterRepository struct{ db *sql.DB }

func NewCashRegisterRepository(db *sql.DB) *CashRegisterRepository {
	return &CashRegisterRepository{db: db}
}

func (r *CashRegisterRepository) List(ctx context.Context, libraryID string, filter models.CashRegisterFilter,
	offset, limit int) ([]models.CashRegister, int, error) {
	where, args := cashRegisterWhere(libraryID, filter)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cash_registers`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count cash registers: %w", err)
	}
	listArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT id,library_id,name,status,version,created_by,created_at,updated_at
		FROM cash_registers`+where+` ORDER BY name COLLATE NOCASE LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list cash registers: %w", err)
	}
	defer rows.Close()
	results := make([]models.CashRegister, 0)
	for rows.Next() {
		var register models.CashRegister
		if err = scanCashRegister(rows, &register); err != nil {
			return nil, 0, fmt.Errorf("scan cash register: %w", err)
		}
		results = append(results, register)
	}
	return results, total, rows.Err()
}

func cashRegisterWhere(libraryID string, filter models.CashRegisterFilter) (string, []interface{}) {
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)
	if libraryID != "" {
		conditions, args = append(conditions, "library_id=?"), append(args, libraryID)
	}
	if filter.Query != "" {
		conditions, args = append(conditions, `name LIKE ? ESCAPE '\'`), append(args, "%"+escapeLike(filter.Query)+"%")
	}
	if filter.Status != "" {
		conditions, args = append(conditions, "status=?"), append(args, filter.Status)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (r *CashRegisterRepository) Find(ctx context.Context, id, libraryID string) (models.CashRegister, error) {
	query := `SELECT id,library_id,name,status,version,created_by,created_at,updated_at FROM cash_registers WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" {
		query, args = query+` AND library_id=?`, append(args, libraryID)
	}
	var register models.CashRegister
	err := scanCashRegister(r.db.QueryRowContext(ctx, query, args...), &register)
	if errors.Is(err, sql.ErrNoRows) {
		return models.CashRegister{}, ErrCashRegisterNotFound
	}
	if err != nil {
		return models.CashRegister{}, fmt.Errorf("find cash register: %w", err)
	}
	return register, nil
}

type cashRegisterScanner interface{ Scan(...interface{}) error }

func scanCashRegister(scanner cashRegisterScanner, register *models.CashRegister) error {
	return scanner.Scan(&register.ID, &register.LibraryID, &register.Name, &register.Status,
		&register.Version, &register.CreatedBy, &register.CreatedAt, &register.UpdatedAt)
}

func (r *CashRegisterRepository) Create(ctx context.Context, register models.CashRegister,
	normalizedName, actorID, auditID, snapshot string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cash register creation: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO cash_registers
		(id,library_id,name,normalized_name,status,version,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)`, register.ID, register.LibraryID, register.Name, normalizedName,
		register.Status, register.Version, register.CreatedBy, register.CreatedAt, register.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrCashRegisterConflict
	}
	if err != nil {
		return fmt.Errorf("insert cash register: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES (?,?,'CREATE_CASH_REGISTER','CASH_REGISTER',?,?,1,?)`, auditID, actorID,
		register.ID, snapshot, register.CreatedAt); err != nil {
		return fmt.Errorf("audit cash register creation: %w", err)
	}
	return tx.Commit()
}

func (r *CashRegisterRepository) Update(ctx context.Context, register models.CashRegister,
	normalizedName string, expectedVersion int, actorID, auditID, oldValues, newValues string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cash register update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE cash_registers SET name=?,normalized_name=?,version=version+1,updated_at=?
		WHERE id=? AND library_id=? AND version=?`, register.Name, normalizedName, register.UpdatedAt,
		register.ID, register.LibraryID, expectedVersion)
	if isUniqueViolation(err) {
		return ErrCashRegisterConflict
	}
	if err != nil {
		return fmt.Errorf("update cash register: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrCashRegisterVersion
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,'UPDATE_CASH_REGISTER','CASH_REGISTER',?,?,?,1,?)`, auditID, actorID,
		register.ID, oldValues, newValues, register.UpdatedAt); err != nil {
		return fmt.Errorf("audit cash register update: %w", err)
	}
	return tx.Commit()
}

func (r *CashRegisterRepository) ChangeStatus(ctx context.Context, register models.CashRegister,
	expected, next models.CashRegisterStatus, expectedVersion int, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cash register status change: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE cash_registers SET status=?,version=version+1,updated_at=?
		WHERE id=? AND library_id=? AND status=? AND version=?`, next, now, register.ID,
		register.LibraryID, expected, expectedVersion)
	if err != nil {
		return fmt.Errorf("change cash register status: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrCashRegisterVersion
	}
	action := "DISABLE_CASH_REGISTER"
	if next == models.CashRegisterStatusActive {
		action = "REACTIVATE_CASH_REGISTER"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,?,'CASH_REGISTER',?,?,?,1,?)`, auditID, actorID, action, register.ID,
		expected, next, now); err != nil {
		return fmt.Errorf("audit cash register status change: %w", err)
	}
	return tx.Commit()
}

func (r *CashRegisterRepository) LibraryActive(ctx context.Context, id string) (bool, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM libraries WHERE id=?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrLibraryUnavailable
	}
	if err != nil {
		return false, fmt.Errorf("find cash register library: %w", err)
	}
	return status == string(models.LibraryStatusActive), nil
}
