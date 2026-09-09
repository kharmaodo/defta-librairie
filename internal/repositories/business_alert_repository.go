package repositories

import (
	"context"
	"database/sql"
)

type BusinessAlert struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resourceId"`
	Label      string `json:"label"`
	Detail     string `json:"detail"`
}
type BusinessAlerts struct {
	Results []BusinessAlert `json:"results"`
	Total   int             `json:"total"`
	Offset  int             `json:"offset"`
	Limit   int             `json:"limit"`
}
type BusinessAlertRepository struct{ db *sql.DB }

func NewBusinessAlertRepository(db *sql.DB) *BusinessAlertRepository {
	return &BusinessAlertRepository{db: db}
}

const businessAlertQuery = `WITH alerts AS (
 SELECT CASE WHEN i.quantity=0 THEN 'OUT_OF_STOCK' ELSE 'LOW_STOCK' END AS kind,
 CAST(i.book_id AS TEXT) AS resource_id, d.title AS label,
 printf('Stock : %d ; seuil : %d',i.quantity,i.low_stock_threshold) AS detail,
 CASE WHEN i.quantity=0 THEN 0 ELSE 1 END AS priority
 FROM book_inventory i JOIN defta d ON d.id=i.book_id
 WHERE i.library_id=? AND d.deleted_at IS NULL AND i.quantity<=i.low_stock_threshold
 UNION ALL
 SELECT 'DRAFT_PURCHASE',p.id,p.reference,'Fournisseur : ' || s.name || CASE WHEN s.status='DISABLED' THEN ' (désactivé)' ELSE '' END,2
 FROM purchases p JOIN suppliers s ON s.id=p.supplier_id AND s.library_id=p.library_id
 WHERE p.library_id=? AND p.status='DRAFT'
 UNION ALL
 SELECT 'DISABLED_SUPPLIER',id,name,'Fournisseur désactivé',3 FROM suppliers WHERE library_id=? AND status='DISABLED'
) `

func (r *BusinessAlertRepository) List(ctx context.Context, library, kind string, offset, limit int) (BusinessAlerts, error) {
	result := BusinessAlerts{Results: []BusinessAlert{}, Offset: offset, Limit: limit}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	args := []interface{}{library, library, library}
	where := ""
	if kind != "" {
		where = " WHERE kind=?"
		args = append(args, kind)
	}
	if err = tx.QueryRowContext(ctx, businessAlertQuery+"SELECT COUNT(*) FROM alerts"+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, businessAlertQuery+"SELECT kind,resource_id,label,detail FROM alerts"+where+" ORDER BY priority,label COLLATE NOCASE,resource_id LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var a BusinessAlert
		if err = rows.Scan(&a.Kind, &a.ResourceID, &a.Label, &a.Detail); err != nil {
			return result, err
		}
		result.Results = append(result.Results, a)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if err = rows.Close(); err != nil {
		return result, err
	}
	return result, tx.Commit()
}
