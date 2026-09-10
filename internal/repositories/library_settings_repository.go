package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"encoding/json"
	"errors"
)

var ErrSettingsNotFound = errors.New("library settings not found")
var ErrSettingsConflict = errors.New("library settings version conflict")

type LibrarySettingsRepository struct{ db *sql.DB }

func NewLibrarySettingsRepository(db *sql.DB) *LibrarySettingsRepository {
	return &LibrarySettingsRepository{db: db}
}
func (r *LibrarySettingsRepository) Find(ctx context.Context, id string, activeOnly bool) (models.LibrarySettings, error) {
	var v models.LibrarySettings
	q := `SELECT s.library_id,l.name,s.currency,s.address,s.phone,s.email,s.logo_data,s.default_low_stock_threshold,s.print_footer,s.version,s.updated_at FROM library_settings s JOIN libraries l ON l.id=s.library_id WHERE s.library_id=?`
	if activeOnly {
		q += " AND l.status='ACTIVE'"
	}
	err := r.db.QueryRowContext(ctx, q, id).Scan(&v.LibraryID, &v.Name, &v.Currency, &v.Address, &v.Phone, &v.Email, &v.LogoData, &v.DefaultLowStockThreshold, &v.PrintFooter, &v.Version, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrSettingsNotFound
	}
	return v, err
}
func (r *LibrarySettingsRepository) Update(ctx context.Context, v models.LibrarySettings, actor, auditID string, activeOnly bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := `UPDATE library_settings SET address=?,phone=?,email=?,logo_data=?,default_low_stock_threshold=?,print_footer=?,version=version+1,updated_at=? WHERE library_id=? AND version=?`
	if activeOnly {
		q += " AND EXISTS(SELECT 1 FROM libraries WHERE id=library_id AND status='ACTIVE')"
	}
	res, err := tx.ExecContext(ctx, q, v.Address, v.Phone, v.Email, v.LogoData, v.DefaultLowStockThreshold, v.PrintFooter, v.UpdatedAt, v.LibraryID, v.Version)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrSettingsConflict
	}
	v.Version++
	v.LogoData = "" // Keep image bytes out of the audit log.
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'UPDATE_LIBRARY_SETTINGS','LIBRARY',?,?,1,?)`, auditID, actor, v.LibraryID, string(payload), v.UpdatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}
