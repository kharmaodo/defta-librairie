package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrExportTooLarge = errors.New("export exceeds 10000 rows; narrow filters")
var ErrInvalidExport = errors.New("invalid export filters")

const ExportRowLimit = 10000

type CSVExportFilter struct{ LibraryID, ActorID, Status, From, To, Search, Action, ResourceType string }
type CSVExportTable struct {
	Header []string
	Rows   [][]string
}
type CSVExportRepository struct{ db *sql.DB }

func NewCSVExportRepository(db *sql.DB) *CSVExportRepository { return &CSVExportRepository{db: db} }

func (r *CSVExportRepository) Read(ctx context.Context, kind string, f CSVExportFilter) (CSVExportTable, error) {
	var t CSVExportTable
	var query, date, status, search, order string
	args := []interface{}{}
	switch kind {
	case "stocks":
		t.Header = []string{"book_id", "title", "quantity", "low_stock_threshold", "average_unit_cost", "updated_at"}
		query = `SELECT i.book_id,d.title,i.quantity,i.low_stock_threshold,i.average_unit_cost,i.updated_at FROM book_inventory i JOIN defta d ON d.id=i.book_id WHERE d.deleted_at IS NULL AND i.library_id=?`
		args = append(args, f.LibraryID)
		date = "i.updated_at"
		status = "CASE WHEN i.quantity=0 THEN 'OUT_OF_STOCK' WHEN i.quantity<=i.low_stock_threshold THEN 'LOW_STOCK' ELSE 'IN_STOCK' END"
		search = "d.title"
		order = "i.book_id"
	case "sales":
		t.Header = []string{"id", "reference", "customer_id", "customer_name", "status", "gross_total", "created_at", "confirmed_at", "cancelled_at"}
		query = `SELECT id,reference,customer_id,customer_name,status,total_amount,created_at,confirmed_at,cancelled_at FROM sales WHERE library_id=?`
		args = append(args, f.LibraryID)
		date = "created_at"
		status = "status"
		search = "reference"
		order = "created_at,id"
	case "purchases":
		t.Header = []string{"id", "reference", "supplier_id", "supplier_name", "status", "gross_total", "created_at", "received_at", "cancelled_at"}
		query = `SELECT p.id,p.reference,p.supplier_id,s.name,p.status,p.total_amount,p.created_at,p.received_at,p.cancelled_at FROM purchases p JOIN suppliers s ON s.id=p.supplier_id AND s.library_id=p.library_id WHERE p.library_id=?`
		args = append(args, f.LibraryID)
		date = "p.created_at"
		status = "p.status"
		search = "p.reference"
		order = "p.created_at,p.id"
	case "suppliers":
		t.Header = []string{"id", "name", "contact_name", "phone", "email", "address", "status", "created_at", "updated_at"}
		query = `SELECT id,name,contact_name,phone,email,address,status,created_at,updated_at FROM suppliers WHERE library_id=?`
		args = append(args, f.LibraryID)
		date = "created_at"
		status = "status"
		search = "name"
		order = "name,id"
	case "audit":
		t.Header = []string{"id", "actor_user_id", "actor_username", "action", "resource_type", "resource_id", "success", "created_at"}
		query = `SELECT a.id,a.actor_user_id,u.username,a.action,a.resource_type,a.resource_id,a.success,a.created_at FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id WHERE 1=1`
		if f.ActorID != "" {
			query += " AND a.actor_user_id=?"
			args = append(args, f.ActorID)
		}
		date = "a.created_at"
		search = "a.resource_id"
		order = "a.created_at,a.id"
	default:
		return t, ErrInvalidExport
	}
	if f.Status != "" {
		if status == "" {
			return t, ErrInvalidExport
		}
		query += " AND (" + status + ")=?"
		args = append(args, f.Status)
	}
	if f.From != "" {
		query += " AND julianday(" + date + ")>=julianday(?)"
		args = append(args, f.From)
	}
	if f.To != "" {
		query += " AND julianday(" + date + ")<julianday(?)"
		args = append(args, f.To)
	}
	if f.Search != "" {
		query += " AND instr(lower(" + search + "),lower(?))>0"
		args = append(args, f.Search)
	}
	for _, p := range []struct{ column, value string }{{"a.action", f.Action}, {"a.resource_type", f.ResourceType}} {
		if p.value != "" {
			if kind != "audit" {
				return t, ErrInvalidExport
			}
			query += " AND " + p.column + "=?"
			args = append(args, p.value)
		}
	}
	rows, err := r.db.QueryContext(ctx, query+" ORDER BY "+order+" LIMIT 10001", args...)
	if err != nil {
		return t, err
	}
	defer rows.Close()
	t.Rows = [][]string{}
	for rows.Next() {
		if len(t.Rows) == ExportRowLimit {
			return CSVExportTable{}, ErrExportTooLarge
		}
		values := make([]interface{}, len(t.Header))
		dest := make([]interface{}, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err = rows.Scan(dest...); err != nil {
			return t, err
		}
		record := make([]string, len(values))
		for i, v := range values {
			switch x := v.(type) {
			case nil:
				record[i] = ""
			case []byte:
				record[i] = string(x)
			default:
				record[i] = fmt.Sprint(x)
			}
		}
		t.Rows = append(t.Rows, record)
	}
	return t, rows.Err()
}
