package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/mattn/go-sqlite3"
)

func TestAuditVisibilityAndFilters(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT NOT NULL);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT, old_values TEXT, new_values TEXT, ip_address TEXT,
			success INTEGER NOT NULL, created_at TEXT NOT NULL
		);
		INSERT INTO users(id, username) VALUES('owner-1', 'owner-one'), ('owner-2', 'owner-two');
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, success, created_at)
		VALUES
			('audit-1', 'owner-1', 'CREATE_BOOK', 'BOOK', '1', 1, '2026-09-02T10:00:00Z'),
			('audit-2', 'owner-2', 'LOGIN_FAILED', 'SESSION', 'session-2', 0, '2026-09-02T11:00:00Z');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	service := NewAuditService(repositories.NewAuditRepository(db))
	ownerClaims := &auth.Claims{Role: models.RoleOwnerLibrary, RegisteredClaims: jwt.RegisteredClaims{Subject: "owner-1"}}
	logs, total, err := service.List(context.Background(), ownerClaims, "", nil, 0, 30)
	if err != nil || total != 1 || len(logs) != 1 || logs[0].ActorUserID != "owner-1" {
		t.Fatalf("owner logs=%+v total=%d err=%v", logs, total, err)
	}
	rootClaims := &auth.Claims{Role: models.RoleSuperAdminRoot, RegisteredClaims: jwt.RegisteredClaims{Subject: "root"}}
	failed := false
	logs, total, err = service.List(context.Background(), rootClaims, "LOGIN_FAILED", &failed, 0, 30)
	if err != nil || total != 1 || len(logs) != 1 || logs[0].ActorUsername != "owner-two" {
		t.Fatalf("root filtered logs=%+v total=%d err=%v", logs, total, err)
	}
	if _, _, err = service.List(context.Background(), rootClaims, "bad action!", nil, 0, 30); err != ErrInvalidAuditFilter {
		t.Fatalf("invalid action error=%v", err)
	}
}
