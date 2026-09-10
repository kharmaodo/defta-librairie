package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"encoding/json"
	"errors"
)

var ErrSupplierReturnStock = errors.New("insufficient stock for supplier return")

// Ship records the transition, inventory changes and audit in one transaction.
func (r *SupplierReturnRepository) Ship(ctx context.Context, id, libraryID string, version int, actor, now string) (models.SupplierReturn, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	defer tx.Rollback()
	// Acquire the write lock before reading inventory. A failed attempt rolls back.
	query := `UPDATE supplier_returns SET status='SHIPPED',shipped_by=?,shipped_at=?,updated_at=?,version=version+1 WHERE id=? AND status='DRAFT' AND version=?`
	args := []interface{}{actor, now, now, id, version}
	if libraryID != "" {
		query += ` AND library_id=?`
		args = append(args, libraryID)
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return models.SupplierReturn{}, err
	}
	if n != 1 {
		q := `SELECT status FROM supplier_returns WHERE id=?`
		a := []interface{}{id}
		if libraryID != "" {
			q += ` AND library_id=?`
			a = append(a, libraryID)
		}
		var status string
		err = tx.QueryRowContext(ctx, q, a...).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return models.SupplierReturn{}, ErrSupplierReturnNotFound
		}
		if err != nil {
			return models.SupplierReturn{}, err
		}
		if status != "DRAFT" {
			return models.SupplierReturn{}, ErrSupplierReturnState
		}
		return models.SupplierReturn{}, ErrSupplierReturnConflict
	}
	var scope, reference string
	if err = tx.QueryRowContext(ctx, `SELECT library_id,reference FROM supplier_returns WHERE id=?`, id).Scan(&scope, &reference); err != nil {
		return models.SupplierReturn{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT book_id,SUM(quantity) FROM supplier_return_lines WHERE return_id=? GROUP BY book_id ORDER BY book_id`, id)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	type line struct {
		book     int64
		quantity int
	}
	lines := []line{}
	for rows.Next() {
		var l line
		if err = rows.Scan(&l.book, &l.quantity); err != nil {
			rows.Close()
			return models.SupplierReturn{}, err
		}
		lines = append(lines, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return models.SupplierReturn{}, err
	}
	if len(lines) == 0 {
		return models.SupplierReturn{}, ErrSupplierReturnLine
	}
	for _, l := range lines {
		var before int
		var cost sql.NullFloat64
		err = tx.QueryRowContext(ctx, `SELECT quantity,average_unit_cost FROM book_inventory WHERE book_id=? AND library_id=?`, l.book, scope).Scan(&before, &cost)
		if errors.Is(err, sql.ErrNoRows) {
			return models.SupplierReturn{}, ErrSupplierReturnLine
		}
		if err != nil {
			return models.SupplierReturn{}, err
		}
		if before < l.quantity {
			return models.SupplierReturn{}, ErrSupplierReturnStock
		}
		// Freeze the current CMP for every returned purchase line of this book.
		// NULL is unknown, whereas zero is a known free cost. The remaining CMP is unchanged.
		_, err = tx.ExecContext(ctx, `UPDATE supplier_return_lines SET unit_cost_snapshot=? WHERE return_id=? AND book_id=?`, cost, id, l.book)
		if err != nil {
			return models.SupplierReturn{}, err
		}
		after := before - l.quantity
		result, err = tx.ExecContext(ctx, `UPDATE book_inventory SET quantity=?,version=version+1,updated_at=? WHERE book_id=? AND library_id=? AND quantity=?`, after, now, l.book, scope, before)
		if err != nil {
			return models.SupplierReturn{}, err
		}
		n, err = result.RowsAffected()
		if err != nil {
			return models.SupplierReturn{}, err
		}
		if n != 1 {
			return models.SupplierReturn{}, ErrSupplierReturnConflict
		}
		movement, err := identity.NewID()
		if err != nil {
			return models.SupplierReturn{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO inventory_movements(id,book_id,library_id,actor_user_id,movement_type,quantity_delta,quantity_before,quantity_after,reason,created_at) VALUES(?,?,?,?,'EXIT',?,?,?,?,?)`, movement, l.book, scope, actor, -l.quantity, before, after, "Retour fournisseur "+reference, now)
		if err != nil {
			return models.SupplierReturn{}, err
		}
		audit, err := identity.NewID()
		if err != nil {
			return models.SupplierReturn{}, err
		}
		payload, err := json.Marshal(map[string]interface{}{"supplierReturnId": id, "quantityBefore": before, "quantityAfter": after, "quantityDelta": -l.quantity, "movementType": "EXIT"})
		if err != nil {
			return models.SupplierReturn{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'UPDATE_INVENTORY','BOOK',?,?,1,?)`, audit, actor, l.book, string(payload), now)
		if err != nil {
			return models.SupplierReturn{}, err
		}
	}
	audit, err := identity.NewID()
	if err != nil {
		return models.SupplierReturn{}, err
	}
	payload, err := json.Marshal(map[string]interface{}{"reference": reference, "from": "DRAFT", "to": "SHIPPED", "version": version + 1})
	if err != nil {
		return models.SupplierReturn{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'SHIP_SUPPLIER_RETURN','SUPPLIER_RETURN',?,?,1,?)`, audit, actor, id, string(payload), now)
	if err != nil {
		return models.SupplierReturn{}, err
	}
	if err = tx.Commit(); err != nil {
		return models.SupplierReturn{}, err
	}
	return r.Find(ctx, id, libraryID)
}
