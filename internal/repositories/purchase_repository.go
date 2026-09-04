package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrPurchaseNotFound = errors.New("purchase not found")
	ErrPurchaseConflict = errors.New("purchase was modified by another request")
	ErrPurchaseState    = errors.New("purchase is not editable")
	ErrPurchaseBook     = errors.New("purchase contains an unavailable book")
	ErrPurchaseSupplier = errors.New("purchase supplier is unavailable")
)

type PurchaseRepository struct{ db *sql.DB }

func NewPurchaseRepository(db *sql.DB) *PurchaseRepository { return &PurchaseRepository{db: db} }

func (r *PurchaseRepository) List(ctx context.Context, libraryID string, filter models.PurchaseFilter,
	offset, limit int) ([]models.Purchase, int, error) {
	where := " WHERE 1=1"
	args := make([]interface{}, 0, 7)
	if libraryID != "" { where += " AND library_id=?"; args = append(args, libraryID) }
	if filter.Status != "" { where += " AND status=?"; args = append(args, filter.Status) }
	if filter.SupplierID != "" { where += " AND supplier_id=?"; args = append(args, filter.SupplierID) }
	if filter.From != "" { where += " AND created_at>=?"; args = append(args, filter.From) }
	if filter.To != "" { where += " AND created_at<=?"; args = append(args, filter.To) }
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM purchases"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count purchases: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT id,library_id,supplier_id,reference,status,total_amount,version,
		created_by,COALESCE(received_by,''),COALESCE(cancelled_by,''),created_at,updated_at,
		COALESCE(received_at,''),COALESCE(cancelled_at,'') FROM purchases`+where+
		` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil { return nil, 0, fmt.Errorf("list purchases: %w", err) }
	purchases := make([]models.Purchase, 0)
	for rows.Next() {
		purchase, scanErr := scanPurchase(rows)
		if scanErr != nil { rows.Close(); return nil, 0, scanErr }
		purchases = append(purchases, purchase)
	}
	if err = rows.Err(); err != nil { rows.Close(); return nil, 0, fmt.Errorf("iterate purchases: %w", err) }
	if err = rows.Close(); err != nil { return nil, 0, fmt.Errorf("close purchases: %w", err) }
	for index := range purchases {
		if purchases[index].Lines, err = r.listLines(ctx, purchases[index].ID); err != nil { return nil, 0, err }
	}
	return purchases, total, nil
}

type purchaseScanner interface{ Scan(...interface{}) error }

func scanPurchase(row purchaseScanner) (models.Purchase, error) {
	var purchase models.Purchase
	err := row.Scan(&purchase.ID, &purchase.LibraryID, &purchase.SupplierID, &purchase.Reference,
		&purchase.Status, &purchase.TotalAmount, &purchase.Version, &purchase.CreatedBy,
		&purchase.ReceivedBy, &purchase.CancelledBy, &purchase.CreatedAt, &purchase.UpdatedAt,
		&purchase.ReceivedAt, &purchase.CancelledAt)
	if err != nil { return models.Purchase{}, fmt.Errorf("scan purchase: %w", err) }
	return purchase, nil
}

func (r *PurchaseRepository) Find(ctx context.Context, id, libraryID string) (models.Purchase, error) {
	query := `SELECT id,library_id,supplier_id,reference,status,total_amount,version,created_by,
		COALESCE(received_by,''),COALESCE(cancelled_by,''),created_at,updated_at,
		COALESCE(received_at,''),COALESCE(cancelled_at,'') FROM purchases WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" { query += " AND library_id=?"; args = append(args, libraryID) }
	purchase, err := scanPurchase(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) || errors.Is(errors.Unwrap(err), sql.ErrNoRows) {
		return models.Purchase{}, ErrPurchaseNotFound
	}
	if err != nil { return models.Purchase{}, err }
	purchase.Lines, err = r.listLines(ctx, purchase.ID)
	return purchase, err
}

func (r *PurchaseRepository) listLines(ctx context.Context, purchaseID string) ([]models.PurchaseLine, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,purchase_id,book_id,title_snapshot,quantity,unit_cost,line_total,created_at
		FROM purchase_lines WHERE purchase_id=? ORDER BY id`, purchaseID)
	if err != nil { return nil, fmt.Errorf("list purchase lines: %w", err) }
	defer rows.Close()
	lines := make([]models.PurchaseLine, 0)
	for rows.Next() {
		var line models.PurchaseLine
		if err = rows.Scan(&line.ID, &line.PurchaseID, &line.BookID, &line.TitleSnapshot,
			&line.Quantity, &line.UnitCost, &line.LineTotal, &line.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan purchase line: %w", err)
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (r *PurchaseRepository) Create(ctx context.Context, purchase models.Purchase, inputs []models.PurchaseLineInput,
	lineIDs []string, auditID, now string) (models.Purchase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return models.Purchase{}, fmt.Errorf("begin purchase creation: %w", err) }
	defer tx.Rollback()
	if err = requireActiveSupplier(ctx, tx, purchase.SupplierID, purchase.LibraryID); err != nil {
		return models.Purchase{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO purchases(id,library_id,supplier_id,reference,status,total_amount,
		version,created_by,created_at,updated_at) VALUES(?,?,?,?,'DRAFT',0,1,?,?,?)`, purchase.ID,
		purchase.LibraryID, purchase.SupplierID, purchase.Reference, purchase.CreatedBy, now, now)
	if err != nil { return models.Purchase{}, fmt.Errorf("insert purchase: %w", err) }
	lines, total, err := writePurchaseLines(ctx, tx, purchase.ID, purchase.LibraryID, inputs, lineIDs, now)
	if err != nil { return models.Purchase{}, err }
	if _, err = tx.ExecContext(ctx, `UPDATE purchases SET total_amount=? WHERE id=?`, total, purchase.ID); err != nil {
		return models.Purchase{}, fmt.Errorf("update purchase total: %w", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{"reference":purchase.Reference,"supplierId":purchase.SupplierID,
		"totalAmount":total,"lines":len(lines),"version":1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,
		new_values,success,created_at) VALUES(?,?,'CREATE_PURCHASE','PURCHASE',?,?,1,?)`, auditID,
		purchase.CreatedBy, purchase.ID, string(payload), now); err != nil {
		return models.Purchase{}, fmt.Errorf("audit purchase creation: %w", err)
	}
	if err = tx.Commit(); err != nil { return models.Purchase{}, fmt.Errorf("commit purchase creation: %w", err) }
	purchase.Status, purchase.TotalAmount, purchase.Version = models.PurchaseStatusDraft, total, 1
	purchase.CreatedAt, purchase.UpdatedAt, purchase.Lines = now, now, lines
	return purchase, nil
}

func requireActiveSupplier(ctx context.Context, tx *sql.Tx, supplierID, libraryID string) error {
	var found int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM suppliers s JOIN libraries l ON l.id=s.library_id
		WHERE s.id=? AND s.library_id=? AND s.status='ACTIVE' AND l.status='ACTIVE'`,
		supplierID, libraryID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) { return ErrPurchaseSupplier }
	if err != nil { return fmt.Errorf("read purchase supplier: %w", err) }
	return nil
}

func writePurchaseLines(ctx context.Context, tx *sql.Tx, purchaseID, libraryID string,
	inputs []models.PurchaseLineInput, lineIDs []string, now string) ([]models.PurchaseLine, float64, error) {
	lines := make([]models.PurchaseLine, 0, len(inputs))
	var total float64
	for index, input := range inputs {
		var title string
		err := tx.QueryRowContext(ctx, `SELECT title FROM defta WHERE id=? AND library_id=? AND deleted_at IS NULL`,
			input.BookID, libraryID).Scan(&title)
		if errors.Is(err, sql.ErrNoRows) { return nil, 0, ErrPurchaseBook }
		if err != nil { return nil, 0, fmt.Errorf("read purchase book: %w", err) }
		lineTotal := input.UnitCost * float64(input.Quantity)
		line := models.PurchaseLine{ID:lineIDs[index], PurchaseID:purchaseID, BookID:input.BookID,
			TitleSnapshot:title, Quantity:input.Quantity, UnitCost:input.UnitCost, LineTotal:lineTotal, CreatedAt:now}
		_, err = tx.ExecContext(ctx, `INSERT INTO purchase_lines(id,purchase_id,book_id,title_snapshot,quantity,
			unit_cost,line_total,created_at) VALUES(?,?,?,?,?,?,?,?)`, line.ID, line.PurchaseID, line.BookID,
			line.TitleSnapshot, line.Quantity, line.UnitCost, line.LineTotal, line.CreatedAt)
		if err != nil { return nil, 0, fmt.Errorf("insert purchase line: %w", err) }
		total += lineTotal
		lines = append(lines, line)
	}
	return lines, total, nil
}

func (r *PurchaseRepository) Update(ctx context.Context, id, libraryID, supplierID string,
	inputs []models.PurchaseLineInput, lineIDs []string, expectedVersion int, actorID, auditID, now string) (models.Purchase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return models.Purchase{}, fmt.Errorf("begin purchase update: %w", err) }
	defer tx.Rollback()
	var actualLibrary string
	var status models.PurchaseStatus
	var version int
	query := `SELECT library_id,status,version FROM purchases WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" { query += ` AND library_id=?`; args = append(args, libraryID) }
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&actualLibrary, &status, &version); errors.Is(err, sql.ErrNoRows) {
		return models.Purchase{}, ErrPurchaseNotFound
	} else if err != nil { return models.Purchase{}, fmt.Errorf("read purchase before update: %w", err) }
	if status != models.PurchaseStatusDraft { return models.Purchase{}, ErrPurchaseState }
	if version != expectedVersion { return models.Purchase{}, ErrPurchaseConflict }
	if err = requireActiveSupplier(ctx, tx, supplierID, actualLibrary); err != nil { return models.Purchase{}, err }
	if _, err = tx.ExecContext(ctx, `DELETE FROM purchase_lines WHERE purchase_id=?`, id); err != nil {
		return models.Purchase{}, fmt.Errorf("replace purchase lines: %w", err)
	}
	_, total, err := writePurchaseLines(ctx, tx, id, actualLibrary, inputs, lineIDs, now)
	if err != nil { return models.Purchase{}, err }
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET supplier_id=?,total_amount=?,version=version+1,updated_at=?
		WHERE id=? AND version=?`, supplierID, total, now, id, expectedVersion)
	if err != nil { return models.Purchase{}, fmt.Errorf("update purchase: %w", err) }
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 { return models.Purchase{}, ErrPurchaseConflict }
	payload, _ := json.Marshal(map[string]interface{}{"supplierId":supplierID,"totalAmount":total,
		"lines":len(inputs),"version":expectedVersion+1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,
		new_values,success,created_at) VALUES(?,?,'UPDATE_PURCHASE','PURCHASE',?,?,1,?)`, auditID,
		actorID, id, string(payload), now); err != nil { return models.Purchase{}, fmt.Errorf("audit purchase update: %w", err) }
	if err = tx.Commit(); err != nil { return models.Purchase{}, fmt.Errorf("commit purchase update: %w", err) }
	return r.Find(ctx, id, libraryID)
}

func (r *PurchaseRepository) Delete(ctx context.Context, id, libraryID string, expectedVersion int,
	actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin purchase deletion: %w", err) }
	defer tx.Rollback()
	query := `SELECT reference,status,total_amount,version FROM purchases WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" { query += ` AND library_id=?`; args = append(args, libraryID) }
	var reference string
	var status models.PurchaseStatus
	var total float64
	var version int
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&reference, &status, &total, &version); errors.Is(err, sql.ErrNoRows) {
		return ErrPurchaseNotFound
	} else if err != nil { return fmt.Errorf("read purchase before deletion: %w", err) }
	if status != models.PurchaseStatusDraft { return ErrPurchaseState }
	if version != expectedVersion { return ErrPurchaseConflict }
	if _, err = tx.ExecContext(ctx, `DELETE FROM purchase_lines WHERE purchase_id=?`, id); err != nil {
		return fmt.Errorf("delete purchase lines: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM purchases WHERE id=? AND status='DRAFT' AND version=?`, id, expectedVersion)
	if err != nil { return fmt.Errorf("delete purchase: %w", err) }
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 { return ErrPurchaseConflict }
	payload, _ := json.Marshal(map[string]interface{}{"reference":reference,"status":status,
		"totalAmount":total,"version":version})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,
		old_values,success,created_at) VALUES(?,?,'DELETE_PURCHASE','PURCHASE',?,?,1,?)`, auditID,
		actorID, id, string(payload), now); err != nil { return fmt.Errorf("audit purchase deletion: %w", err) }
	return tx.Commit()
}
