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
	ErrInventoryNotFound  = errors.New("inventory not found")
	ErrInventoryConflict  = errors.New("inventory was modified by another request")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrInventoryUnchanged = errors.New("inventory quantity is unchanged")
)

type InventoryRepository struct{ db *sql.DB }

func NewInventoryRepository(db *sql.DB) *InventoryRepository { return &InventoryRepository{db: db} }

func (r *InventoryRepository) Find(ctx context.Context, bookID int, libraryID string) (models.BookInventory, error) {
	query := `SELECT i.book_id, i.library_id, i.quantity, i.low_stock_threshold, i.version, i.updated_at
		FROM book_inventory i JOIN defta d ON d.id=i.book_id
		WHERE i.book_id=? AND d.deleted_at IS NULL`
	args := []interface{}{bookID}
	if libraryID != "" {
		query += ` AND i.library_id=?`
		args = append(args, libraryID)
	}
	var inventory models.BookInventory
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&inventory.BookID, &inventory.LibraryID,
		&inventory.Quantity, &inventory.LowStockThreshold, &inventory.Version, &inventory.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BookInventory{}, ErrInventoryNotFound
	}
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("find inventory: %w", err)
	}
	return inventory, nil
}

func (r *InventoryRepository) ApplyMovement(ctx context.Context, bookID int, libraryID, actorID, movementID,
	auditID string, movementType models.InventoryMovementType, quantity, expectedVersion int, reason, now string) (models.BookInventory, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("begin inventory movement: %w", err)
	}
	defer tx.Rollback()
	query := `SELECT i.quantity, i.low_stock_threshold, i.version, i.library_id
		FROM book_inventory i JOIN defta d ON d.id=i.book_id
		WHERE i.book_id=? AND d.deleted_at IS NULL`
	args := []interface{}{bookID}
	if libraryID != "" {
		query += ` AND i.library_id=?`
		args = append(args, libraryID)
	}
	var before, threshold, version int
	var actualLibrary string
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&before, &threshold, &version, &actualLibrary); errors.Is(err, sql.ErrNoRows) {
		return models.BookInventory{}, ErrInventoryNotFound
	} else if err != nil {
		return models.BookInventory{}, fmt.Errorf("read inventory before movement: %w", err)
	}
	if version != expectedVersion {
		return models.BookInventory{}, ErrInventoryConflict
	}
	after := quantity
	if movementType == models.InventoryMovementEntry {
		after = before + quantity
	} else if movementType == models.InventoryMovementExit {
		after = before - quantity
	}
	if after < 0 {
		return models.BookInventory{}, ErrInsufficientStock
	}
	if after == before {
		return models.BookInventory{}, ErrInventoryUnchanged
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE book_inventory SET quantity=?, version=version+1, updated_at=?
		WHERE book_id=? AND version=?
	`, after, now, bookID, expectedVersion)
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("update inventory: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return models.BookInventory{}, ErrInventoryConflict
	}
	delta := after - before
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO inventory_movements(id, book_id, library_id, actor_user_id, movement_type,
			quantity_delta, quantity_before, quantity_after, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
	`, movementID, bookID, actualLibrary, actorID, movementType, delta, before, after, reason, now); err != nil {
		return models.BookInventory{}, fmt.Errorf("insert inventory movement: %w", err)
	}
	payload, err := json.Marshal(map[string]interface{}{"movementType": movementType, "quantityBefore": before,
		"quantityAfter": after, "quantityDelta": delta, "version": expectedVersion + 1})
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("encode inventory audit: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'UPDATE_INVENTORY', 'BOOK', ?, ?, 1, ?)
	`, auditID, actorID, bookID, string(payload), now); err != nil {
		return models.BookInventory{}, fmt.Errorf("audit inventory movement: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.BookInventory{}, fmt.Errorf("commit inventory movement: %w", err)
	}
	return models.BookInventory{BookID: int64(bookID), LibraryID: actualLibrary, Quantity: after,
		LowStockThreshold: threshold, Version: expectedVersion + 1, UpdatedAt: now}, nil
}

func (r *InventoryRepository) UpdateThreshold(ctx context.Context, bookID int, libraryID, actorID, auditID string,
	threshold, expectedVersion int, now string) (models.BookInventory, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("begin inventory threshold update: %w", err)
	}
	defer tx.Rollback()
	query := `SELECT i.quantity, i.low_stock_threshold, i.version, i.library_id
		FROM book_inventory i JOIN defta d ON d.id=i.book_id
		WHERE i.book_id=? AND d.deleted_at IS NULL`
	args := []interface{}{bookID}
	if libraryID != "" {
		query += ` AND i.library_id=?`
		args = append(args, libraryID)
	}
	var quantity, previousThreshold, version int
	var actualLibrary string
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&quantity, &previousThreshold, &version, &actualLibrary); errors.Is(err, sql.ErrNoRows) {
		return models.BookInventory{}, ErrInventoryNotFound
	} else if err != nil {
		return models.BookInventory{}, fmt.Errorf("read inventory before threshold update: %w", err)
	}
	if version != expectedVersion {
		return models.BookInventory{}, ErrInventoryConflict
	}
	if previousThreshold == threshold {
		return models.BookInventory{}, ErrInventoryUnchanged
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE book_inventory SET low_stock_threshold=?, version=version+1, updated_at=?
		WHERE book_id=? AND version=?
	`, threshold, now, bookID, expectedVersion)
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("update inventory threshold: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return models.BookInventory{}, ErrInventoryConflict
	}
	payload, err := json.Marshal(map[string]interface{}{
		"lowStockThresholdBefore": previousThreshold,
		"lowStockThresholdAfter":  threshold,
		"version":                 expectedVersion + 1,
	})
	if err != nil {
		return models.BookInventory{}, fmt.Errorf("encode inventory threshold audit: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'UPDATE_INVENTORY_THRESHOLD', 'BOOK', ?, ?, 1, ?)
	`, auditID, actorID, bookID, string(payload), now); err != nil {
		return models.BookInventory{}, fmt.Errorf("audit inventory threshold update: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.BookInventory{}, fmt.Errorf("commit inventory threshold update: %w", err)
	}
	return models.BookInventory{BookID: int64(bookID), LibraryID: actualLibrary, Quantity: quantity,
		LowStockThreshold: threshold, Version: expectedVersion + 1, UpdatedAt: now}, nil
}

func (r *InventoryRepository) ListMovements(ctx context.Context, bookID int, libraryID string, offset, limit int) ([]models.InventoryMovement, int, error) {
	where := ` WHERE i.book_id=? AND d.deleted_at IS NULL`
	args := []interface{}{bookID}
	if libraryID != "" {
		where += ` AND i.library_id=?`
		args = append(args, libraryID)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM inventory_movements i
		JOIN defta d ON d.id=i.book_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count inventory movements: %w", err)
	}
	if total == 0 {
		var exists int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM book_inventory i
			JOIN defta d ON d.id=i.book_id`+where, args...).Scan(&exists); err != nil {
			return nil, 0, fmt.Errorf("check inventory before listing movements: %w", err)
		}
		if exists == 0 {
			return nil, 0, ErrInventoryNotFound
		}
	}
	queryArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, `SELECT i.id, i.book_id, i.library_id, i.actor_user_id,
		i.movement_type, i.quantity_delta, i.quantity_before, i.quantity_after, COALESCE(i.reason, ''), i.created_at
		FROM inventory_movements i JOIN defta d ON d.id=i.book_id`+where+`
		ORDER BY i.created_at DESC, i.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list inventory movements: %w", err)
	}
	defer rows.Close()
	movements := make([]models.InventoryMovement, 0)
	for rows.Next() {
		var movement models.InventoryMovement
		if err = rows.Scan(&movement.ID, &movement.BookID, &movement.LibraryID, &movement.ActorUserID,
			&movement.MovementType, &movement.QuantityDelta, &movement.QuantityBefore, &movement.QuantityAfter,
			&movement.Reason, &movement.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan inventory movement: %w", err)
		}
		movements = append(movements, movement)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate inventory movements: %w", err)
	}
	return movements, total, nil
}
