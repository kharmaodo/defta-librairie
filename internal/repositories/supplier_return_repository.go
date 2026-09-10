package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSupplierReturnNotFound = errors.New("supplier return not found")
	ErrSupplierReturnConflict = errors.New("supplier return was modified by another request")
	ErrSupplierReturnState    = errors.New("supplier return is not editable")
	ErrSupplierReturnPurchase = errors.New("supplier return purchase is unavailable")
	ErrSupplierReturnLine     = errors.New("supplier return line is unavailable")
	ErrSupplierReturnQuantity = errors.New("supplier return quantity exceeds received quantity")
)

type SupplierReturnRepository struct{ db *sql.DB }

func NewSupplierReturnRepository(db *sql.DB) *SupplierReturnRepository {
	return &SupplierReturnRepository{db: db}
}

const supplierReturnSelect = `SELECT id,library_id,purchase_id,supplier_id,reference,
	COALESCE(supplier_reference,''),reason,status,total_amount,version,created_by,
	COALESCE(shipped_by,''),COALESCE(cancelled_by,''),created_at,updated_at,
	COALESCE(shipped_at,''),COALESCE(cancelled_at,'') FROM supplier_returns`

type supplierReturnScanner interface{ Scan(...interface{}) error }

func scanSupplierReturn(row supplierReturnScanner) (models.SupplierReturn, error) {
	var value models.SupplierReturn
	err := row.Scan(&value.ID, &value.LibraryID, &value.PurchaseID, &value.SupplierID,
		&value.Reference, &value.SupplierReference, &value.Reason, &value.Status,
		&value.TotalAmount, &value.Version, &value.CreatedBy, &value.ShippedBy,
		&value.CancelledBy, &value.CreatedAt, &value.UpdatedAt, &value.ShippedAt,
		&value.CancelledAt)
	return value, err
}

func (r *SupplierReturnRepository) List(ctx context.Context, libraryID string,
	filter models.SupplierReturnFilter, offset, limit int) ([]models.SupplierReturn, int, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	if libraryID != "" {
		where += " AND library_id=?"
		args = append(args, libraryID)
	}
	if filter.Status != "" {
		where += " AND status=?"
		args = append(args, filter.Status)
	}
	if filter.PurchaseID != "" {
		where += " AND purchase_id=?"
		args = append(args, filter.PurchaseID)
	}
	if filter.SupplierID != "" {
		where += " AND supplier_id=?"
		args = append(args, filter.SupplierID)
	}
	if filter.From != "" {
		where += " AND created_at>=?"
		args = append(args, filter.From)
	}
	if filter.To != "" {
		where += " AND created_at<=?"
		args = append(args, filter.To)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM supplier_returns"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, supplierReturnSelect+where+" ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	values := []models.SupplierReturn{}
	for rows.Next() {
		value, scanErr := scanSupplierReturn(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		values = append(values, value)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	// Release the connection before querying child rows, including with a one-connection pool.
	if err = rows.Close(); err != nil {
		return nil, 0, err
	}
	for i := range values {
		values[i].Lines, err = r.listLines(ctx, values[i].ID)
		if err != nil {
			return nil, 0, err
		}
	}
	return values, total, nil
}

func (r *SupplierReturnRepository) Find(ctx context.Context, id, libraryID string) (models.SupplierReturn, error) {
	query, args := supplierReturnSelect+" WHERE id=?", []interface{}{id}
	if libraryID != "" {
		query += " AND library_id=?"
		args = append(args, libraryID)
	}
	value, err := scanSupplierReturn(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return value, ErrSupplierReturnNotFound
	}
	if err != nil {
		return value, err
	}
	value.Lines, err = r.listLines(ctx, id)
	return value, err
}

func (r *SupplierReturnRepository) listLines(ctx context.Context, id string) ([]models.SupplierReturnLine, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,return_id,purchase_line_id,book_id,title_snapshot,quantity,unit_cost,line_total,created_at,unit_cost_snapshot,quantity*unit_cost_snapshot,line_total-quantity*unit_cost_snapshot FROM supplier_return_lines WHERE return_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []models.SupplierReturnLine{}
	for rows.Next() {
		var value models.SupplierReturnLine
		if err = rows.Scan(&value.ID, &value.ReturnID, &value.PurchaseLineID, &value.BookID, &value.Title, &value.Quantity, &value.UnitCost, &value.LineTotal, &value.CreatedAt, &value.UnitCostSnapshot, &value.InventoryCost, &value.CostVariance); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r *SupplierReturnRepository) Create(ctx context.Context, value models.SupplierReturn,
	inputs []models.SupplierReturnLineInput, lineIDs []string, auditID, now string) (models.SupplierReturn, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return value, err
	}
	defer tx.Rollback()
	var supplierID string
	if err = tx.QueryRowContext(ctx, `SELECT supplier_id FROM purchases WHERE id=? AND library_id=? AND status='RECEIVED'`, value.PurchaseID, value.LibraryID).Scan(&supplierID); errors.Is(err, sql.ErrNoRows) {
		return value, ErrSupplierReturnPurchase
	} else if err != nil {
		return value, err
	}
	value.SupplierID = supplierID
	_, err = tx.ExecContext(ctx, `INSERT INTO supplier_returns(id,library_id,purchase_id,supplier_id,reference,supplier_reference,reason,status,total_amount,version,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'DRAFT',0,1,?,?,?)`, value.ID, value.LibraryID, value.PurchaseID, value.SupplierID, value.Reference, nullable(value.SupplierReference), value.Reason, value.CreatedBy, now, now)
	if err != nil {
		return value, mapSupplierReturnError(err)
	}
	lines, total, err := writeSupplierReturnLines(ctx, tx, value.ID, value.PurchaseID, inputs, lineIDs, now)
	if err != nil {
		return value, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE supplier_returns SET total_amount=? WHERE id=?", total, value.ID); err != nil {
		return value, err
	}
	payload, _ := json.Marshal(map[string]interface{}{"reference": value.Reference, "purchaseId": value.PurchaseID, "totalAmount": total, "lines": len(lines), "version": 1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'CREATE_SUPPLIER_RETURN','SUPPLIER_RETURN',?,?,1,?)`, auditID, value.CreatedBy, value.ID, string(payload), now); err != nil {
		return value, err
	}
	if err = tx.Commit(); err != nil {
		return value, err
	}
	value.Status = models.SupplierReturnStatusDraft
	value.TotalAmount = total
	value.Version = 1
	value.CreatedAt = now
	value.UpdatedAt = now
	value.Lines = lines
	return value, nil
}

func writeSupplierReturnLines(ctx context.Context, tx *sql.Tx, returnID, purchaseID string, inputs []models.SupplierReturnLineInput, lineIDs []string, now string) ([]models.SupplierReturnLine, float64, error) {
	values := make([]models.SupplierReturnLine, 0, len(inputs))
	var total float64
	for index, input := range inputs {
		var bookID int64
		var title string
		var unitCost float64
		err := tx.QueryRowContext(ctx, `SELECT book_id,title_snapshot,unit_cost FROM purchase_lines WHERE id=? AND purchase_id=?`, input.PurchaseLineID, purchaseID).Scan(&bookID, &title, &unitCost)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, ErrSupplierReturnLine
		}
		if err != nil {
			return nil, 0, err
		}
		lineTotal := unitCost * float64(input.Quantity)
		value := models.SupplierReturnLine{ID: lineIDs[index], ReturnID: returnID, PurchaseLineID: input.PurchaseLineID, BookID: bookID, Title: title, Quantity: input.Quantity, UnitCost: unitCost, LineTotal: lineTotal, CreatedAt: now}
		if _, err = tx.ExecContext(ctx, `INSERT INTO supplier_return_lines(id,return_id,purchase_line_id,book_id,title_snapshot,quantity,unit_cost,line_total,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, value.ID, value.ReturnID, value.PurchaseLineID, value.BookID, value.Title, value.Quantity, value.UnitCost, value.LineTotal, value.CreatedAt); err != nil {
			return nil, 0, mapSupplierReturnError(err)
		}
		values = append(values, value)
		total += lineTotal
	}
	return values, total, nil
}

func mapSupplierReturnError(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "quantity exceeds"):
		return ErrSupplierReturnQuantity
	case strings.Contains(message, "purchase unavailable"):
		return ErrSupplierReturnPurchase
	case strings.Contains(message, "line unavailable"):
		return ErrSupplierReturnLine
	case strings.Contains(message, "unique"):
		return ErrSupplierReturnConflict
	default:
		return fmt.Errorf("supplier return: %w", err)
	}
}

func (r *SupplierReturnRepository) Update(ctx context.Context, id, libraryID, supplierReference, reason string,
	inputs []models.SupplierReturnLineInput, lineIDs []string, expectedVersion int,
	actorID, auditID, now string) (models.SupplierReturn, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	defer tx.Rollback()
	query, args := `SELECT purchase_id,status,version FROM supplier_returns WHERE id=?`, []interface{}{id}
	if libraryID != "" {
		query += ` AND library_id=?`
		args = append(args, libraryID)
	}
	var purchaseID string
	var status models.SupplierReturnStatus
	var version int
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&purchaseID, &status, &version); errors.Is(err, sql.ErrNoRows) {
		return models.SupplierReturn{}, ErrSupplierReturnNotFound
	} else if err != nil {
		return models.SupplierReturn{}, err
	}
	if status != models.SupplierReturnStatusDraft {
		return models.SupplierReturn{}, ErrSupplierReturnState
	}
	if version != expectedVersion {
		return models.SupplierReturn{}, ErrSupplierReturnConflict
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM supplier_return_lines WHERE return_id=?`, id); err != nil {
		return models.SupplierReturn{}, err
	}
	_, total, err := writeSupplierReturnLines(ctx, tx, id, purchaseID, inputs, lineIDs, now)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE supplier_returns SET supplier_reference=?,reason=?,total_amount=?,version=version+1,updated_at=? WHERE id=? AND status='DRAFT' AND version=?`, nullable(supplierReference), reason, total, now, id, expectedVersion)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return models.SupplierReturn{}, ErrSupplierReturnConflict
	}
	payload, _ := json.Marshal(map[string]interface{}{"supplierReference": supplierReference, "reason": reason, "totalAmount": total, "lines": len(inputs), "version": expectedVersion + 1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'UPDATE_SUPPLIER_RETURN','SUPPLIER_RETURN',?,?,1,?)`, auditID, actorID, id, string(payload), now); err != nil {
		return models.SupplierReturn{}, err
	}
	if err = tx.Commit(); err != nil {
		return models.SupplierReturn{}, err
	}
	return r.Find(ctx, id, libraryID)
}

func (r *SupplierReturnRepository) Cancel(ctx context.Context, id, libraryID string, expectedVersion int, actorID, auditID, now string) (models.SupplierReturn, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	defer tx.Rollback()
	query, args := `SELECT reference,status,version FROM supplier_returns WHERE id=?`, []interface{}{id}
	if libraryID != "" {
		query += ` AND library_id=?`
		args = append(args, libraryID)
	}
	var reference string
	var status models.SupplierReturnStatus
	var version int
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&reference, &status, &version); errors.Is(err, sql.ErrNoRows) {
		return models.SupplierReturn{}, ErrSupplierReturnNotFound
	} else if err != nil {
		return models.SupplierReturn{}, err
	}
	if status != models.SupplierReturnStatusDraft {
		return models.SupplierReturn{}, ErrSupplierReturnState
	}
	if version != expectedVersion {
		return models.SupplierReturn{}, ErrSupplierReturnConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE supplier_returns SET status='CANCELLED',cancelled_by=?,cancelled_at=?,version=version+1,updated_at=? WHERE id=? AND status='DRAFT' AND version=?`, actorID, now, now, id, expectedVersion)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return models.SupplierReturn{}, ErrSupplierReturnConflict
	}
	payload, _ := json.Marshal(map[string]interface{}{"reference": reference, "from": "DRAFT", "to": "CANCELLED", "version": expectedVersion + 1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'CANCEL_SUPPLIER_RETURN','SUPPLIER_RETURN',?,?,1,?)`, auditID, actorID, id, string(payload), now); err != nil {
		return models.SupplierReturn{}, err
	}
	if err = tx.Commit(); err != nil {
		return models.SupplierReturn{}, err
	}
	return r.Find(ctx, id, libraryID)
}
