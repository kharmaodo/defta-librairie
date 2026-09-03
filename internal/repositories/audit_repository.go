package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"fmt"
)

type AuditRepository struct{ db *sql.DB }

func NewAuditRepository(db *sql.DB) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) List(ctx context.Context, actorID string, filter models.AuditFilter, offset, limit int) ([]models.AuditLog, int, error) {
	where := " WHERE 1=1"
	args := make([]interface{}, 0, 10)
	if actorID != "" {
		where += " AND a.actor_user_id=?"
		args = append(args, actorID)
	}
	if filter.ActorUsername != "" {
		where += " AND u.username LIKE ?"
		args = append(args, "%"+filter.ActorUsername+"%")
	}
	if filter.Action != "" {
		where += " AND a.action=?"
		args = append(args, filter.Action)
	}
	if filter.ResourceType != "" {
		where += " AND a.resource_type=?"
		args = append(args, filter.ResourceType)
	}
	if filter.ResourceID != "" {
		where += " AND a.resource_id=?"
		args = append(args, filter.ResourceID)
	}
	if filter.Success != nil {
		where += " AND a.success=?"
		args = append(args, *filter.Success)
	}
	if filter.From != "" {
		where += " AND a.created_at>=?"
		args = append(args, filter.From)
	}
	if filter.To != "" {
		where += " AND a.created_at<=?"
		args = append(args, filter.To)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, COALESCE(a.actor_user_id, ''), COALESCE(u.username, ''), a.action,
		       a.resource_type, COALESCE(a.resource_id, ''), COALESCE(a.old_values, ''),
		       COALESCE(a.new_values, ''), COALESCE(a.ip_address, ''), a.success, a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id
	`+where+" ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()
	logs := make([]models.AuditLog, 0)
	for rows.Next() {
		var log models.AuditLog
		if err = rows.Scan(&log.ID, &log.ActorUserID, &log.ActorUsername, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.OldValues, &log.NewValues,
			&log.IPAddress, &log.Success, &log.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}
