package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/mattn/go-sqlite3"
)

func TestActiveSessionScopeAndRevocation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "active-sessions.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT NOT NULL, role TEXT NOT NULL);
		CREATE TABLE refresh_sessions (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL, token_family TEXT NOT NULL,
			expires_at TEXT NOT NULL, revoked_at TEXT, replaced_by_id TEXT, ip_address TEXT,
			user_agent TEXT, created_at TEXT NOT NULL, last_used_at TEXT
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT, new_values TEXT, success INTEGER NOT NULL, created_at TEXT NOT NULL
		);
		INSERT INTO users(id, username, role) VALUES
		('owner-1', 'owner-one', 'OWNER_LIBRARY'), ('owner-2', 'owner-two', 'OWNER_LIBRARY'), ('root', 'root-admin', 'SUPER_ADMIN_ROOT');
		INSERT INTO refresh_sessions(id, user_id, token_hash, token_family, expires_at, ip_address, user_agent, created_at)
		VALUES
			('session-1', 'owner-1', 'hash-1', 'family-1', '2099-01-01T00:00:00Z', '192.0.2.1', 'Browser One', '2026-09-02T10:00:00Z'),
			('session-1-other', 'owner-1', 'hash-1-other', 'family-1-other', '2099-01-01T00:00:00Z', '192.0.2.3', 'Browser Other', '2026-09-02T10:30:00Z'),
			('session-2', 'owner-2', 'hash-2', 'family-2', '2099-01-01T00:00:00Z', '192.0.2.2', 'Browser Two', '2026-09-02T11:00:00Z');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	tokens, err := auth.NewTokenManager("01234567890123456789012345678901", "issuer", "audience", time.Minute)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	service, err := NewSessionService(repositories.NewSessionRepository(db), tokens, time.Hour)
	if err != nil {
		t.Fatalf("session service: %v", err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) }
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, SessionID: "session-1", RegisteredClaims: jwt.RegisteredClaims{Subject: "owner-1"}}
	sessions, total, err := service.ListActive(context.Background(), owner, models.SessionFilter{IPAddress: "192.0.2.1"}, 0, 30)
	if err != nil || total != 1 || len(sessions) != 1 || sessions[0].ID != "session-1" {
		t.Fatalf("owner sessions=%+v total=%d err=%v", sessions, total, err)
	}
	if err = service.RevokeActive(context.Background(), owner, "session-2"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("cross-account revocation error=%v", err)
	}
	revokedOthers, err := service.RevokeOthers(context.Background(), owner)
	if err != nil || revokedOthers != 1 {
		t.Fatalf("revoke other sessions=%d err=%v", revokedOthers, err)
	}
	var currentActive, otherRevoked, otherAudits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM refresh_sessions WHERE id='session-1' AND revoked_at IS NULL`).Scan(&currentActive)
	_ = db.QueryRow(`SELECT COUNT(*) FROM refresh_sessions WHERE id='session-1-other' AND revoked_at IS NOT NULL`).Scan(&otherRevoked)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='OTHER_SESSIONS_REVOKED' AND actor_user_id='owner-1' AND new_values='{"revoked_families":1}'`).Scan(&otherAudits)
	if currentActive != 1 || otherRevoked != 1 || otherAudits != 1 {
		t.Fatalf("currentActive=%d otherRevoked=%d otherAudits=%d", currentActive, otherRevoked, otherAudits)
	}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot, RegisteredClaims: jwt.RegisteredClaims{Subject: "root"}}
	sessions, total, err = service.ListActive(context.Background(), root, models.SessionFilter{
		Username: "owner-two", Role: "owner_library", UserAgent: "Browser Two",
	}, 0, 30)
	if err != nil || total != 1 || len(sessions) != 1 || sessions[0].ID != "session-2" {
		t.Fatalf("root filtered sessions=%+v total=%d err=%v", sessions, total, err)
	}
	if _, _, err = service.ListActive(context.Background(), owner, models.SessionFilter{Username: "owner-two"}, 0, 30); !errors.Is(err, ErrInvalidSessionFilter) {
		t.Fatalf("owner username filter error=%v", err)
	}
	if err = service.RevokeActive(context.Background(), root, "session-2"); err != nil {
		t.Fatalf("root revocation: %v", err)
	}
	var revoked, audits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM refresh_sessions WHERE id='session-2' AND revoked_at IS NOT NULL`).Scan(&revoked)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='SESSION_REVOKED' AND actor_user_id='root'`).Scan(&audits)
	if revoked != 1 || audits != 1 {
		t.Fatalf("revoked=%d audits=%d", revoked, audits)
	}
}
