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
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrPaymentSaleNotFound  = errors.New("payment sale not found")
	ErrPaymentConflict      = errors.New("payment reference conflict")
	ErrPaymentVersion       = errors.New("payment version conflict")
	ErrPaymentState         = errors.New("payment state conflict")
	ErrPaymentOverpaid      = errors.New("payment exceeds remaining amount")
	ErrPaymentUnavailable   = errors.New("payment context unavailable")
)

type PaymentRepository struct{ db *sql.DB }

func NewPaymentRepository(db *sql.DB) *PaymentRepository { return &PaymentRepository{db: db} }

func (r *PaymentRepository) SaleLibrary(ctx context.Context, saleID, libraryID string) (string, error) {
	query, args := `SELECT library_id FROM sales WHERE id=?`, []interface{}{saleID}
	if libraryID != "" { query, args = query+` AND library_id=?`, append(args, libraryID) }
	var result string
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) { return "", ErrPaymentSaleNotFound }
	if err != nil { return "", fmt.Errorf("find payment sale: %w", err) }
	return result, nil
}

func (r *PaymentRepository) List(ctx context.Context, saleID, libraryID string, filter models.PaymentFilter,
	offset, limit int) ([]models.Payment, int, error) {
	where, args := paymentWhere(saleID, libraryID, filter)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count payments: %w", err)
	}
	listArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT id,library_id,sale_id,cash_register_id,method,amount,
		COALESCE(external_reference,''),COALESCE(notes,''),status,version,recorded_by,
		COALESCE(voided_by,''),created_at,updated_at,COALESCE(voided_at,'')
		FROM payments`+where+` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil { return nil, 0, fmt.Errorf("list payments: %w", err) }
	defer rows.Close()
	results := make([]models.Payment, 0)
	for rows.Next() {
		var payment models.Payment
		if err = scanPayment(rows, &payment); err != nil { return nil, 0, fmt.Errorf("scan payment: %w", err) }
		results = append(results, payment)
	}
	return results, total, rows.Err()
}

func paymentWhere(saleID, libraryID string, filter models.PaymentFilter) (string, []interface{}) {
	conditions, args := []string{"sale_id=?"}, []interface{}{saleID}
	if libraryID != "" { conditions, args = append(conditions, "library_id=?"), append(args, libraryID) }
	if filter.Method != "" { conditions, args = append(conditions, "method=?"), append(args, filter.Method) }
	if filter.Status != "" { conditions, args = append(conditions, "status=?"), append(args, filter.Status) }
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (r *PaymentRepository) Find(ctx context.Context, id, libraryID string) (models.Payment, error) {
	query := `SELECT id,library_id,sale_id,cash_register_id,method,amount,COALESCE(external_reference,''),
		COALESCE(notes,''),status,version,recorded_by,COALESCE(voided_by,''),created_at,updated_at,
		COALESCE(voided_at,'') FROM payments WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" { query, args = query+` AND library_id=?`, append(args, libraryID) }
	var payment models.Payment
	err := scanPayment(r.db.QueryRowContext(ctx, query, args...), &payment)
	if errors.Is(err, sql.ErrNoRows) { return models.Payment{}, ErrPaymentNotFound }
	if err != nil { return models.Payment{}, fmt.Errorf("find payment: %w", err) }
	return payment, nil
}

type paymentScanner interface{ Scan(...interface{}) error }

func scanPayment(scanner paymentScanner, payment *models.Payment) error {
	return scanner.Scan(&payment.ID, &payment.LibraryID, &payment.SaleID, &payment.CashRegisterID,
		&payment.Method, &payment.Amount, &payment.ExternalReference, &payment.Notes, &payment.Status,
		&payment.Version, &payment.RecordedBy, &payment.VoidedBy, &payment.CreatedAt,
		&payment.UpdatedAt, &payment.VoidedAt)
}

func (r *PaymentRepository) Balance(ctx context.Context, saleID, libraryID string) (models.SalePaymentBalance, error) {
	query := `SELECT sale_id,library_id,total_amount,paid_amount,remaining_amount,payment_status
		FROM sale_payment_balances WHERE sale_id=?`
	args := []interface{}{saleID}
	if libraryID != "" { query, args = query+` AND library_id=?`, append(args, libraryID) }
	var balance models.SalePaymentBalance
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&balance.SaleID, &balance.LibraryID,
		&balance.TotalAmount, &balance.PaidAmount, &balance.RemainingAmount, &balance.PaymentStatus)
	if errors.Is(err, sql.ErrNoRows) { return models.SalePaymentBalance{}, ErrPaymentSaleNotFound }
	if err != nil { return models.SalePaymentBalance{}, fmt.Errorf("find payment balance: %w", err) }
	return balance, nil
}

func (r *PaymentRepository) Create(ctx context.Context, payment models.Payment, auditID, snapshot string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin payment creation: %w", err) }
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO payments
		(id,library_id,sale_id,cash_register_id,method,amount,external_reference,notes,status,version,
		recorded_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, payment.ID,
		payment.LibraryID, payment.SaleID, payment.CashRegisterID, payment.Method, payment.Amount,
		payment.ExternalReference, payment.Notes, payment.Status, payment.Version, payment.RecordedBy,
		payment.CreatedAt, payment.UpdatedAt)
	if err != nil {
		message := strings.ToLower(err.Error())
		switch {
		case strings.Contains(message, "unique"):
			return ErrPaymentConflict
		case strings.Contains(message, "exceeds sale"):
			return ErrPaymentOverpaid
		case strings.Contains(message, "unavailable"):
			return ErrPaymentUnavailable
		default:
			return fmt.Errorf("insert payment: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES (?,?,'RECORD_PAYMENT','PAYMENT',?,?,1,?)`, auditID, payment.RecordedBy,
		payment.ID, snapshot, payment.CreatedAt); err != nil { return fmt.Errorf("audit payment creation: %w", err) }
	return tx.Commit()
}

func (r *PaymentRepository) Void(ctx context.Context, payment models.Payment, expectedVersion int,
	reason, actorID, auditID, oldValues, newValues, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin payment void: %w", err) }
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE payments SET status='VOIDED',voided_by=?,voided_at=?,
		notes=CASE WHEN ?='' THEN notes WHEN notes IS NULL OR notes='' THEN ? ELSE notes || ' | ' || ? END,
		version=version+1,updated_at=? WHERE id=? AND library_id=? AND status='RECORDED' AND version=?`,
		actorID, now, reason, reason, reason, now, payment.ID, payment.LibraryID, expectedVersion)
	if err != nil { return fmt.Errorf("void payment: %w", err) }
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 { return ErrPaymentVersion }
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,'VOID_PAYMENT','PAYMENT',?,?,?,1,?)`, auditID, actorID, payment.ID,
		oldValues, newValues, now); err != nil { return fmt.Errorf("audit payment void: %w", err) }
	return tx.Commit()
}
