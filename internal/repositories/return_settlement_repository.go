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
	ErrReturnSettlementUnfunded       = errors.New("monetary refund exceeds recorded payments")
	ErrReturnSettlementNotFound       = errors.New("return settlement not found")
	ErrReturnSettlementReturnNotFound = errors.New("customer return not found")
	ErrReturnSettlementConflict       = errors.New("return settlement reference conflict")
	ErrReturnSettlementVersion        = errors.New("return settlement version conflict")
	ErrReturnSettlementState          = errors.New("return settlement state conflict")
	ErrReturnSettlementOverpaid       = errors.New("return settlement exceeds remaining amount")
	ErrReturnSettlementUnavailable    = errors.New("return settlement context unavailable")
)

type ReturnSettlementRepository struct{ db *sql.DB }

func NewReturnSettlementRepository(db *sql.DB) *ReturnSettlementRepository {
	return &ReturnSettlementRepository{db: db}
}

func (r *ReturnSettlementRepository) ReturnLibrary(ctx context.Context, returnID, libraryID string) (string, error) {
	q := `SELECT library_id FROM customer_returns WHERE id=?`
	args := []interface{}{returnID}
	if libraryID != "" {
		q += " AND library_id=?"
		args = append(args, libraryID)
	}
	var value string
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrReturnSettlementReturnNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find customer return: %w", err)
	}
	return value, nil
}
func (r *ReturnSettlementRepository) List(ctx context.Context, returnID, libraryID string, filter models.ReturnSettlementFilter, offset, limit int) ([]models.ReturnSettlement, int, error) {
	where := " WHERE return_id=?"
	args := []interface{}{returnID}
	if libraryID != "" {
		where += " AND library_id=?"
		args = append(args, libraryID)
	}
	if filter.Method != "" {
		where += " AND method=?"
		args = append(args, filter.Method)
	}
	if filter.Status != "" {
		where += " AND status=?"
		args = append(args, filter.Status)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM return_settlements"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, settlementSelect+where+` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	values := []models.ReturnSettlement{}
	for rows.Next() {
		var v models.ReturnSettlement
		if err = scanReturnSettlement(rows, &v); err != nil {
			return nil, 0, err
		}
		values = append(values, v)
	}
	return values, total, rows.Err()
}

const settlementSelect = `SELECT id,library_id,return_id,method,amount,COALESCE(external_reference,''),COALESCE(notes,''),status,version,issued_by,COALESCE(voided_by,''),created_at,updated_at,COALESCE(voided_at,'') FROM return_settlements`

type settlementScanner interface{ Scan(...interface{}) error }

func scanReturnSettlement(row settlementScanner, v *models.ReturnSettlement) error {
	return row.Scan(&v.ID, &v.LibraryID, &v.ReturnID, &v.Method, &v.Amount, &v.ExternalReference, &v.Notes, &v.Status, &v.Version, &v.IssuedBy, &v.VoidedBy, &v.CreatedAt, &v.UpdatedAt, &v.VoidedAt)
}
func (r *ReturnSettlementRepository) Find(ctx context.Context, id, libraryID string) (models.ReturnSettlement, error) {
	q := settlementSelect + ` WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" {
		q += " AND library_id=?"
		args = append(args, libraryID)
	}
	var v models.ReturnSettlement
	err := scanReturnSettlement(r.db.QueryRowContext(ctx, q, args...), &v)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrReturnSettlementNotFound
	}
	if err != nil {
		return v, err
	}
	return v, nil
}
func (r *ReturnSettlementRepository) Balance(ctx context.Context, returnID, libraryID string) (models.CustomerReturnBalance, error) {
	q := `SELECT return_id,library_id,total_amount,settled_amount,remaining_amount,settlement_status FROM customer_return_balances WHERE return_id=?`
	args := []interface{}{returnID}
	if libraryID != "" {
		q += " AND library_id=?"
		args = append(args, libraryID)
	}
	var v models.CustomerReturnBalance
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&v.ReturnID, &v.LibraryID, &v.TotalAmount, &v.SettledAmount, &v.RemainingAmount, &v.SettlementStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrReturnSettlementReturnNotFound
	}
	return v, err
}
func (r *ReturnSettlementRepository) Create(ctx context.Context, v models.ReturnSettlement, auditID, snapshot string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO return_settlements(id,library_id,return_id,method,amount,external_reference,notes,status,version,issued_by,created_at,updated_at)VALUES(?,?,?,?,?,?,?,'ISSUED',1,?,?,?)`, v.ID, v.LibraryID, v.ReturnID, v.Method, v.Amount, nullable(v.ExternalReference), nullable(v.Notes), v.IssuedBy, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return mapSettlementError(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)VALUES(?,?,'ISSUE_RETURN_SETTLEMENT','RETURN_SETTLEMENT',?,?,1,?)`, auditID, v.IssuedBy, v.ID, snapshot, v.CreatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *ReturnSettlementRepository) Void(ctx context.Context, v models.ReturnSettlement, expected int, reason, actor, auditID, oldValues, newValues, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE return_settlements SET status='VOIDED',voided_by=?,voided_at=?,notes=CASE WHEN ?='' THEN notes WHEN notes IS NULL OR notes='' THEN ? ELSE notes||' | '||? END,version=version+1,updated_at=? WHERE id=? AND library_id=? AND status='ISSUED' AND version=?`, actor, now, reason, reason, reason, now, v.ID, v.LibraryID, expected)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrReturnSettlementVersion
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)VALUES(?,?,'VOID_RETURN_SETTLEMENT','RETURN_SETTLEMENT',?,?,?,1,?)`, auditID, actor, v.ID, oldValues, newValues, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func mapSettlementError(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "monetary refund exceeds recorded payments"):
		return ErrReturnSettlementUnfunded
	case strings.Contains(message, "unique"):
		return ErrReturnSettlementConflict
	case strings.Contains(message, "exceeds return"):
		return ErrReturnSettlementOverpaid
	case strings.Contains(message, "unavailable"):
		return ErrReturnSettlementUnavailable
	default:
		return fmt.Errorf("return settlement: %w", err)
	}
}
