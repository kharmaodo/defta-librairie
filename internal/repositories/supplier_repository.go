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
	ErrSupplierNotFound = errors.New("supplier not found")
	ErrSupplierConflict = errors.New("supplier already exists")
	ErrSupplierVersion  = errors.New("supplier version conflict")
	ErrSupplierState    = errors.New("supplier state conflict")
)

type SupplierRepository struct{ db *sql.DB }

func NewSupplierRepository(db *sql.DB) *SupplierRepository { return &SupplierRepository{db: db} }

func (r *SupplierRepository) List(ctx context.Context, libraryID, query string, status models.SupplierStatus,
	offset, limit int) ([]models.Supplier, int, error) {
	where, args := supplierWhere(libraryID, query, status)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM suppliers`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count suppliers: %w", err)
	}
	listArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, library_id, name, COALESCE(contact_name,''), COALESCE(phone,''),
		       COALESCE(email,''), COALESCE(address,''), status, version, created_by, created_at, updated_at
		FROM suppliers`+where+` ORDER BY name COLLATE NOCASE LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list suppliers: %w", err)
	}
	defer rows.Close()
	suppliers := make([]models.Supplier, 0)
	for rows.Next() {
		var supplier models.Supplier
		if err = scanSupplier(rows, &supplier); err != nil {
			return nil, 0, fmt.Errorf("scan supplier: %w", err)
		}
		suppliers = append(suppliers, supplier)
	}
	return suppliers, total, rows.Err()
}

func supplierWhere(libraryID, query string, status models.SupplierStatus) (string, []interface{}) {
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)
	if libraryID != "" {
		conditions, args = append(conditions, "library_id=?"), append(args, libraryID)
	}
	if query != "" {
		conditions, args = append(conditions, `(name LIKE ? ESCAPE '\' OR contact_name LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\')`),
			append(args, "%"+escapeLike(query)+"%", "%"+escapeLike(query)+"%", "%"+escapeLike(query)+"%")
	}
	if status != "" {
		conditions, args = append(conditions, "status=?"), append(args, status)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func (r *SupplierRepository) Find(ctx context.Context, id, libraryID string) (models.Supplier, error) {
	query := `SELECT id, library_id, name, COALESCE(contact_name,''), COALESCE(phone,''),
		COALESCE(email,''), COALESCE(address,''), status, version, created_by, created_at, updated_at
		FROM suppliers WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" {
		query, args = query+` AND library_id=?`, append(args, libraryID)
	}
	var supplier models.Supplier
	err := scanSupplier(r.db.QueryRowContext(ctx, query, args...), &supplier)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Supplier{}, ErrSupplierNotFound
	}
	if err != nil {
		return models.Supplier{}, fmt.Errorf("find supplier: %w", err)
	}
	return supplier, nil
}

type supplierScanner interface{ Scan(...interface{}) error }

func scanSupplier(scanner supplierScanner, supplier *models.Supplier) error {
	return scanner.Scan(&supplier.ID, &supplier.LibraryID, &supplier.Name, &supplier.ContactName,
		&supplier.Phone, &supplier.Email, &supplier.Address, &supplier.Status, &supplier.Version,
		&supplier.CreatedBy, &supplier.CreatedAt, &supplier.UpdatedAt)
}

func (r *SupplierRepository) Create(ctx context.Context, supplier models.Supplier, normalizedName, actorID, auditID, snapshot string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin supplier creation: %w", err) }
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO suppliers
		(id,library_id,name,normalized_name,contact_name,phone,email,address,status,version,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, supplier.ID, supplier.LibraryID, supplier.Name, normalizedName, supplier.ContactName,
		supplier.Phone, supplier.Email, supplier.Address, supplier.Status, supplier.Version,
		supplier.CreatedBy, supplier.CreatedAt, supplier.UpdatedAt)
	if isUniqueViolation(err) { return ErrSupplierConflict }
	if err != nil { return fmt.Errorf("insert supplier: %w", err) }
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES (?,?,'CREATE_SUPPLIER','SUPPLIER',?,?,1,?)`, auditID, actorID, supplier.ID, snapshot, supplier.CreatedAt); err != nil {
		return fmt.Errorf("audit supplier creation: %w", err)
	}
	return tx.Commit()
}

func (r *SupplierRepository) Update(ctx context.Context, supplier models.Supplier, expectedVersion int,
	actorID, auditID, oldValues, newValues string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin supplier update: %w", err) }
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE suppliers SET name=?,normalized_name=?,contact_name=?,phone=?,email=?,address=?,
		version=version+1,updated_at=? WHERE id=? AND library_id=? AND version=?`, supplier.Name,
		strings.ToLower(supplier.Name), supplier.ContactName, supplier.Phone, supplier.Email, supplier.Address, supplier.UpdatedAt,
		supplier.ID, supplier.LibraryID, expectedVersion)
	if isUniqueViolation(err) { return ErrSupplierConflict }
	if err != nil { return fmt.Errorf("update supplier: %w", err) }
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 { return ErrSupplierVersion }
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,'UPDATE_SUPPLIER','SUPPLIER',?,?,?,1,?)`, auditID, actorID, supplier.ID,
		oldValues, newValues, supplier.UpdatedAt); err != nil { return fmt.Errorf("audit supplier update: %w", err) }
	return tx.Commit()
}

func (r *SupplierRepository) ChangeStatus(ctx context.Context, supplier models.Supplier, expected, next models.SupplierStatus,
	expectedVersion int, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin supplier status change: %w", err) }
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE suppliers SET status=?,version=version+1,updated_at=?
		WHERE id=? AND library_id=? AND status=? AND version=?`, next, now, supplier.ID, supplier.LibraryID, expected, expectedVersion)
	if err != nil { return fmt.Errorf("change supplier status: %w", err) }
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		if supplier.Status != expected { return ErrSupplierState }
		return ErrSupplierVersion
	}
	action := "DISABLE_SUPPLIER"
	if next == models.SupplierStatusActive { action = "REACTIVATE_SUPPLIER" }
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,?,'SUPPLIER',?,?,?,1,?)`, auditID, actorID, action, supplier.ID, expected, next, now); err != nil {
		return fmt.Errorf("audit supplier status change: %w", err)
	}
	return tx.Commit()
}

func (r *SupplierRepository) LibraryActive(ctx context.Context, id string) (bool, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM libraries WHERE id=?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) { return false, ErrLibraryUnavailable }
	if err != nil { return false, fmt.Errorf("find supplier library: %w", err) }
	return status == string(models.LibraryStatusActive), nil
}
