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
	ErrCustomerNotFound = errors.New("customer not found")
	ErrCustomerConflict = errors.New("customer already exists")
	ErrCustomerVersion  = errors.New("customer version conflict")
	ErrCustomerState    = errors.New("customer state conflict")
)

type CustomerRepository struct{ db *sql.DB }

func NewCustomerRepository(db *sql.DB) *CustomerRepository { return &CustomerRepository{db: db} }

func (r *CustomerRepository) List(ctx context.Context, libraryID string, filter models.CustomerFilter,
	offset, limit int) ([]models.Customer, int, error) {
	where, args := customerWhere(libraryID, filter)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM customers`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count customers: %w", err)
	}
	listArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id,library_id,reference,name,COALESCE(phone,''),COALESCE(email,''),
		       COALESCE(address,''),COALESCE(notes,''),status,version,created_by,created_at,updated_at
		FROM customers`+where+` ORDER BY name COLLATE NOCASE, reference LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list customers: %w", err)
	}
	defer rows.Close()
	customers := make([]models.Customer, 0)
	for rows.Next() {
		var customer models.Customer
		if err = scanCustomer(rows, &customer); err != nil {
			return nil, 0, fmt.Errorf("scan customer: %w", err)
		}
		customers = append(customers, customer)
	}
	return customers, total, rows.Err()
}

func customerWhere(libraryID string, filter models.CustomerFilter) (string, []interface{}) {
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 5)
	if libraryID != "" {
		conditions, args = append(conditions, "library_id=?"), append(args, libraryID)
	}
	if filter.Query != "" {
		value := "%" + escapeLike(filter.Query) + "%"
		conditions = append(conditions, `(reference LIKE ? ESCAPE '\' OR name LIKE ? ESCAPE '\' OR phone LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\')`)
		args = append(args, value, value, value, value)
	}
	if filter.Status != "" {
		conditions, args = append(conditions, "status=?"), append(args, filter.Status)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (r *CustomerRepository) Find(ctx context.Context, id, libraryID string) (models.Customer, error) {
	query := `SELECT id,library_id,reference,name,COALESCE(phone,''),COALESCE(email,''),
		COALESCE(address,''),COALESCE(notes,''),status,version,created_by,created_at,updated_at
		FROM customers WHERE id=?`
	args := []interface{}{id}
	if libraryID != "" {
		query, args = query+` AND library_id=?`, append(args, libraryID)
	}
	var customer models.Customer
	err := scanCustomer(r.db.QueryRowContext(ctx, query, args...), &customer)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Customer{}, ErrCustomerNotFound
	}
	if err != nil {
		return models.Customer{}, fmt.Errorf("find customer: %w", err)
	}
	return customer, nil
}

type customerScanner interface{ Scan(...interface{}) error }

func scanCustomer(scanner customerScanner, customer *models.Customer) error {
	return scanner.Scan(&customer.ID, &customer.LibraryID, &customer.Reference, &customer.Name,
		&customer.Phone, &customer.Email, &customer.Address, &customer.Notes, &customer.Status,
		&customer.Version, &customer.CreatedBy, &customer.CreatedAt, &customer.UpdatedAt)
}

func (r *CustomerRepository) Create(ctx context.Context, customer models.Customer, actorID, auditID, snapshot string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin customer creation: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO customers
		(id,library_id,reference,name,phone,email,address,notes,status,version,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, customer.ID, customer.LibraryID, customer.Reference, customer.Name,
		customer.Phone, customer.Email, customer.Address, customer.Notes, customer.Status, customer.Version,
		customer.CreatedBy, customer.CreatedAt, customer.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrCustomerConflict
	}
	if err != nil {
		return fmt.Errorf("insert customer: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES (?,?,'CREATE_CUSTOMER','CUSTOMER',?,?,1,?)`, auditID, actorID, customer.ID, snapshot, customer.CreatedAt); err != nil {
		return fmt.Errorf("audit customer creation: %w", err)
	}
	return tx.Commit()
}

func (r *CustomerRepository) Update(ctx context.Context, customer models.Customer, expectedVersion int,
	actorID, auditID, oldValues, newValues string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin customer update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE customers SET name=?,phone=?,email=?,address=?,notes=?,
		version=version+1,updated_at=? WHERE id=? AND library_id=? AND version=?`, customer.Name,
		customer.Phone, customer.Email, customer.Address, customer.Notes, customer.UpdatedAt,
		customer.ID, customer.LibraryID, expectedVersion)
	if err != nil {
		return fmt.Errorf("update customer: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrCustomerVersion
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,'UPDATE_CUSTOMER','CUSTOMER',?,?,?,1,?)`, auditID, actorID, customer.ID,
		oldValues, newValues, customer.UpdatedAt); err != nil {
		return fmt.Errorf("audit customer update: %w", err)
	}
	return tx.Commit()
}

func (r *CustomerRepository) ChangeStatus(ctx context.Context, customer models.Customer,
	expected, next models.CustomerStatus, expectedVersion int, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin customer status change: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE customers SET status=?,version=version+1,updated_at=?
		WHERE id=? AND library_id=? AND status=? AND version=?`, next, now, customer.ID,
		customer.LibraryID, expected, expectedVersion)
	if err != nil {
		return fmt.Errorf("change customer status: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		if customer.Status != expected {
			return ErrCustomerState
		}
		return ErrCustomerVersion
	}
	action := "DISABLE_CUSTOMER"
	if next == models.CustomerStatusActive {
		action = "REACTIVATE_CUSTOMER"
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs
		(id,actor_user_id,action,resource_type,resource_id,old_values,new_values,success,created_at)
		VALUES (?,?,?,'CUSTOMER',?,?,?,1,?)`, auditID, actorID, action, customer.ID, expected, next, now); err != nil {
		return fmt.Errorf("audit customer status change: %w", err)
	}
	return tx.Commit()
}

func (r *CustomerRepository) LibraryActive(ctx context.Context, id string) (bool, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM libraries WHERE id=?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrLibraryUnavailable
	}
	if err != nil {
		return false, fmt.Errorf("find customer library: %w", err)
	}
	return status == string(models.LibraryStatusActive), nil
}
