package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
)

// SaleHistory must be called with the library of the authorized customer.
func (r *CustomerRepository) SaleHistory(ctx context.Context, customerID, libraryID string, filter models.CustomerHistoryFilter, offset, limit int) ([]models.CustomerSaleSummary, int, error) {
	where := " WHERE customer_id=? AND library_id=?"
	args := []interface{}{customerID, libraryID}
	if filter.Status == "" {
		where += " AND status IN ('CONFIRMED','CANCELLED')"
	} else {
		where += " AND status=?"
		args = append(args, filter.Status)
	}
	event := `julianday(CASE WHEN status='DRAFT' THEN created_at ELSE confirmed_at END)`
	if filter.From != "" {
		where += " AND " + event + ">=julianday(?)"
		args = append(args, filter.From)
	}
	if filter.To != "" {
		where += " AND " + event + "<julianday(?)"
		args = append(args, filter.To)
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var total int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM sales"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,reference,status,total_amount,created_at,COALESCE(confirmed_at,''),COALESCE(cancelled_at,'') FROM sales`+where+" ORDER BY "+event+" DESC,id DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	values := []models.CustomerSaleSummary{}
	for rows.Next() {
		var v models.CustomerSaleSummary
		if err = rows.Scan(&v.ID, &v.Reference, &v.Status, &v.TotalAmount, &v.CreatedAt, &v.ConfirmedAt, &v.CancelledAt); err != nil {
			return nil, 0, err
		}
		values = append(values, v)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	if err = rows.Close(); err != nil {
		return nil, 0, err
	}
	if err = tx.Commit(); err != nil {
		return nil, 0, err
	}
	return values, total, nil
}
