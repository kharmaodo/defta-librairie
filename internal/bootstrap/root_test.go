package bootstrap

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateRoot(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "bootstrap.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			email TEXT UNIQUE COLLATE NOCASE, password_hash TEXT NOT NULL,
			role TEXT NOT NULL, status TEXT NOT NULL, failed_login_attempts INTEGER NOT NULL DEFAULT 0,
			locked_until TEXT, last_login_at TEXT, password_changed_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE UNIQUE INDEX idx_users_single_super_admin_root ON users(role) WHERE role = 'SUPER_ADMIN_ROOT';
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT, old_values TEXT, new_values TEXT, ip_address TEXT, success INTEGER NOT NULL, created_at TEXT NOT NULL
		);
		CREATE TABLE refresh_sessions (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			token_family TEXT NOT NULL, expires_at TEXT NOT NULL, revoked_at TEXT,
			replaced_by_id TEXT, ip_address TEXT, user_agent TEXT, created_at TEXT NOT NULL, last_used_at TEXT
		);
		CREATE TABLE password_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT NOT NULL,
			password_hash TEXT NOT NULL, created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	input := RootInput{Username: "kharmaodo", Email: "root@example.com", Password: "Correct-Horse-2026"}
	user, err := CreateRoot(context.Background(), db, input)
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	if user.Role != models.RoleSuperAdminRoot || user.Status != models.UserStatusActive {
		t.Fatalf("unexpected root: role=%s status=%s", user.Role, user.Status)
	}
	valid, err := auth.VerifyPassword(input.Password, user.PasswordHash)
	if err != nil || !valid {
		t.Fatalf("verify stored password: valid=%v err=%v", valid, err)
	}

	var auditCount int
	if err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action = 'BOOTSTRAP_SUPER_ADMIN' AND resource_id = ?", user.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit entries: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one audit entry, got %d", auditCount)
	}

	_, err = CreateRoot(context.Background(), db, input)
	if !errors.Is(err, ErrRootAlreadyExists) {
		t.Fatalf("expected ErrRootAlreadyExists, got %v", err)
	}
}

func TestResetRootPassword(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "reset.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, email TEXT,
		password_hash TEXT NOT NULL, role TEXT NOT NULL, status TEXT NOT NULL,
		failed_login_attempts INTEGER NOT NULL DEFAULT 0, locked_until TEXT, last_login_at TEXT,
		password_changed_at TEXT, must_change_password INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
		CREATE TABLE audit_logs (id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT NOT NULL,
		resource_type TEXT NOT NULL, resource_id TEXT, old_values TEXT, new_values TEXT,
		ip_address TEXT, success INTEGER NOT NULL, created_at TEXT NOT NULL);
		CREATE TABLE refresh_sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
		token_family TEXT NOT NULL, expires_at TEXT NOT NULL, revoked_at TEXT, replaced_by_id TEXT,
		ip_address TEXT, user_agent TEXT, created_at TEXT NOT NULL, last_used_at TEXT);
		CREATE TABLE password_history (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT NOT NULL,
		password_hash TEXT NOT NULL, created_at TEXT NOT NULL);
		INSERT INTO users(id,username,password_hash,role,status,failed_login_attempts,locked_until,created_at,updated_at)
		VALUES('root','kharmaodo','old-hash','SUPER_ADMIN_ROOT','LOCKED',5,'2099-01-01T00:00:00Z','now','now');
		INSERT INTO refresh_sessions(id,user_id,token_hash,token_family,expires_at,created_at)
		VALUES('session','root','token-hash','family','2099-01-01T00:00:00Z','now');
	`)
	if err != nil {
		t.Fatalf("create reset schema: %v", err)
	}

	t.Setenv("DEFTA_ROOT_NEW_PASSWORD", "New-Correct-Horse-2026")
	user, err := ResetRootPasswordFromEnvironment(context.Background(), db)
	if err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if user.Status != models.UserStatusActive || user.FailedLoginAttempts != 0 {
		t.Fatalf("unexpected reset user: %+v", user)
	}

	var hash, status string
	var attempts int
	if err = db.QueryRow("SELECT password_hash,status,failed_login_attempts FROM users WHERE id='root'").Scan(&hash, &status, &attempts); err != nil {
		t.Fatal(err)
	}
	valid, err := auth.VerifyPassword("New-Correct-Horse-2026", hash)
	if err != nil || !valid || status != "ACTIVE" || attempts != 0 {
		t.Fatalf("invalid reset state")
	}
	var revoked, audits int
	_ = db.QueryRow("SELECT COUNT(*) FROM refresh_sessions WHERE revoked_at IS NOT NULL").Scan(&revoked)
	_ = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='RESET_ROOT_PASSWORD'").Scan(&audits)
	if revoked != 1 || audits != 1 {
		t.Fatalf("revoked=%d audits=%d", revoked, audits)
	}
}
