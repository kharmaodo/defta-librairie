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
	ErrSaleNotFound = errors.New("sale not found")
	ErrSaleConflict = errors.New("sale was modified by another request")
	ErrSaleState    = errors.New("sale is not editable")
	ErrSaleBook     = errors.New("sale contains an unavailable book")
)

type SaleRepository struct{ db *sql.DB }

func NewSaleRepository(db *sql.DB) *SaleRepository { return &SaleRepository{db: db} }

func (r *SaleRepository) List(ctx context.Context, libraryID string, filter models.SaleFilter,
	offset, limit int) ([]models.Sale, int, error) {
	where := " WHERE 1=1"
	args := make([]interface{}, 0, 6)
	if libraryID != "" {
		where += " AND library_id=?"
		args = append(args, libraryID)
	}
	if filter.Status != "" {
		where += " AND status=?"
		args = append(args, filter.Status)
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
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sales"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sales: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT id, library_id, reference, COALESCE(customer_name,''),
		status, total_amount, version, created_by, COALESCE(confirmed_by,''), COALESCE(cancelled_by,''),
		created_at, updated_at, COALESCE(confirmed_at,''), COALESCE(cancelled_at,'')
		FROM sales`+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sales: %w", err)
	}
	defer rows.Close()
	sales := make([]models.Sale, 0)
	for rows.Next() {
		sale, scanErr := scanSale(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		sale.Lines = make([]models.SaleLine, 0)
		sales = append(sales, sale)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sales: %w", err)
	}
	return sales, total, nil
}

type saleScanner interface{ Scan(...interface{}) error }

func scanSale(row saleScanner) (models.Sale, error) {
	var sale models.Sale
	if err := row.Scan(&sale.ID, &sale.LibraryID, &sale.Reference, &sale.CustomerName, &sale.Status,
		&sale.TotalAmount, &sale.Version, &sale.CreatedBy, &sale.ConfirmedBy, &sale.CancelledBy,
		&sale.CreatedAt, &sale.UpdatedAt, &sale.ConfirmedAt, &sale.CancelledAt); err != nil {
		return models.Sale{}, fmt.Errorf("scan sale: %w", err)
	}
	return sale, nil
}

func (r *SaleRepository) Find(ctx context.Context, id, libraryID string) (models.Sale, error) {
	query := `SELECT id, library_id, reference, COALESCE(customer_name,''),
		status, total_amount, version, created_by, COALESCE(confirmed_by,''), COALESCE(cancelled_by,''),
		created_at, updated_at, COALESCE(confirmed_at,''), COALESCE(cancelled_at,'')
		FROM sales WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" {
		query += " AND library_id=?"
		args = append(args, libraryID)
	}
	sale, err := scanSale(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) || errors.Is(errors.Unwrap(err), sql.ErrNoRows) {
		return models.Sale{}, ErrSaleNotFound
	}
	if err != nil {
		return models.Sale{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, sale_id, book_id, title_snapshot, quantity,
		unit_price, line_total, created_at FROM sale_lines WHERE sale_id=? ORDER BY id`, id)
	if err != nil {
		return models.Sale{}, fmt.Errorf("list sale lines: %w", err)
	}
	defer rows.Close()
	sale.Lines = make([]models.SaleLine, 0)
	for rows.Next() {
		var line models.SaleLine
		if err = rows.Scan(&line.ID, &line.SaleID, &line.BookID, &line.TitleSnapshot, &line.Quantity,
			&line.UnitPrice, &line.LineTotal, &line.CreatedAt); err != nil {
			return models.Sale{}, fmt.Errorf("scan sale line: %w", err)
		}
		sale.Lines = append(sale.Lines, line)
	}
	return sale, rows.Err()
}

func (r *SaleRepository) Create(ctx context.Context, sale models.Sale, inputs []models.SaleLineInput,
	lineIDs []string, auditID, now string) (models.Sale, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Sale{}, fmt.Errorf("begin sale creation: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO sales(id,library_id,reference,customer_name,status,
		total_amount,version,created_by,created_at,updated_at)
		VALUES(?,?,?,NULLIF(?,''),'DRAFT',0,1,?,?,?)`,
		sale.ID, sale.LibraryID, sale.Reference, sale.CustomerName, sale.CreatedBy, now, now); err != nil {
		return models.Sale{}, fmt.Errorf("insert sale: %w", err)
	}
	lines, total, err := writeSaleLines(ctx, tx, sale.ID, sale.LibraryID, inputs, lineIDs, now)
	if err != nil {
		return models.Sale{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sales SET total_amount=? WHERE id=?", total, sale.ID); err != nil {
		return models.Sale{}, fmt.Errorf("update sale total: %w", err)
	}
	payload, _ := json.Marshal(map[string]interface{}{"reference": sale.Reference, "totalAmount": total, "lines": len(lines), "version": 1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES(?,?,'CREATE_SALE','SALE',?,?,1,?)`, auditID, sale.CreatedBy, sale.ID, string(payload), now); err != nil {
		return models.Sale{}, fmt.Errorf("audit sale creation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.Sale{}, fmt.Errorf("commit sale creation: %w", err)
	}
	sale.Status, sale.TotalAmount, sale.Version = models.SaleStatusDraft, total, 1
	sale.CreatedAt, sale.UpdatedAt, sale.Lines = now, now, lines
	return sale, nil
}

func writeSaleLines(ctx context.Context, tx *sql.Tx, saleID, libraryID string, inputs []models.SaleLineInput,
	lineIDs []string, now string) ([]models.SaleLine, float64, error) {
	lines := make([]models.SaleLine, 0, len(inputs))
	var total float64
	for index, input := range inputs {
		var title string
		var price float64
		err := tx.QueryRowContext(ctx, `SELECT title, price FROM defta
			WHERE id=? AND library_id=? AND deleted_at IS NULL`, input.BookID, libraryID).Scan(&title, &price)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, ErrSaleBook
		}
		if err != nil {
			return nil, 0, fmt.Errorf("read sale book: %w", err)
		}
		lineTotal := price * float64(input.Quantity)
		line := models.SaleLine{ID: lineIDs[index], SaleID: saleID, BookID: input.BookID,
			TitleSnapshot: title, Quantity: input.Quantity, UnitPrice: price, LineTotal: lineTotal, CreatedAt: now}
		if _, err = tx.ExecContext(ctx, `INSERT INTO sale_lines(id,sale_id,book_id,title_snapshot,
			quantity,unit_price,line_total,created_at) VALUES(?,?,?,?,?,?,?,?)`,
			line.ID, saleID, line.BookID, line.TitleSnapshot, line.Quantity, line.UnitPrice, line.LineTotal, now); err != nil {
			return nil, 0, fmt.Errorf("insert sale line: %w", err)
		}
		total += lineTotal
		lines = append(lines, line)
	}
	return lines, total, nil
}

func (r *SaleRepository) Update(ctx context.Context, id, libraryID, customer string,
	inputs []models.SaleLineInput, lineIDs []string, expectedVersion int, actorID, auditID, now string) (models.Sale, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Sale{}, fmt.Errorf("begin sale update: %w", err)
	}
	defer tx.Rollback()
	var status models.SaleStatus
	var version int
	query := "SELECT status,version FROM sales WHERE id=?"
	args := []interface{}{id}
	if libraryID != "" {
		query += " AND library_id=?"
		args = append(args, libraryID)
	}
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&status, &version); errors.Is(err, sql.ErrNoRows) {
		return models.Sale{}, ErrSaleNotFound
	} else if err != nil {
		return models.Sale{}, fmt.Errorf("read sale before update: %w", err)
	}
	if status != models.SaleStatusDraft {
		return models.Sale{}, ErrSaleState
	}
	if version != expectedVersion {
		return models.Sale{}, ErrSaleConflict
	}
	var actualLibrary string
	if err = tx.QueryRowContext(ctx, "SELECT library_id FROM sales WHERE id=?", id).Scan(&actualLibrary); err != nil {
		return models.Sale{}, fmt.Errorf("read sale library: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sale_lines WHERE sale_id=?", id); err != nil {
		return models.Sale{}, fmt.Errorf("replace sale lines: %w", err)
	}
	_, total, err := writeSaleLines(ctx, tx, id, actualLibrary, inputs, lineIDs, now)
	if err != nil {
		return models.Sale{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE sales SET customer_name=NULLIF(?,''),total_amount=?,
		version=version+1,updated_at=? WHERE id=? AND version=?`, customer, total, now, id, expectedVersion)
	if err != nil {
		return models.Sale{}, fmt.Errorf("update sale: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return models.Sale{}, ErrSaleConflict
	}
	payload, _ := json.Marshal(map[string]interface{}{"totalAmount": total, "lines": len(inputs), "version": expectedVersion + 1})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES(?,?,'UPDATE_SALE','SALE',?,?,1,?)`, auditID, actorID, id, string(payload), now); err != nil {
		return models.Sale{}, fmt.Errorf("audit sale update: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.Sale{}, fmt.Errorf("commit sale update: %w", err)
	}
	return r.Find(ctx, id, libraryID)
}
