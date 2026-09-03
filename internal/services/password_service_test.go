package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestPasswordChangeRevokesSessionsAndAudits(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "password.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY, username TEXT NOT NULL, email TEXT, password_hash TEXT NOT NULL,
			role TEXT NOT NULL, status TEXT NOT NULL, failed_login_attempts INTEGER NOT NULL DEFAULT 0,
			locked_until TEXT, password_changed_at TEXT, must_change_password INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE libraries (id TEXT PRIMARY KEY, owner_user_id TEXT, status TEXT);
		CREATE TABLE refresh_sessions (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL, token_family TEXT NOT NULL,
			expires_at TEXT NOT NULL, revoked_at TEXT, created_at TEXT NOT NULL
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT, new_values TEXT, ip_address TEXT, success INTEGER NOT NULL, created_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	oldPassword := "Old-Password-2026"
	newPassword := "New-Password-2026"
	oldHash, err := auth.HashPassword(oldPassword)
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id, username, password_hash, role, status, must_change_password, created_at, updated_at)
		VALUES('user-1', 'owner', ?, 'OWNER_LIBRARY', 'ACTIVE', 1, 'now', 'now');
		INSERT INTO libraries(id, owner_user_id, status) VALUES('library-1', 'user-1', 'ACTIVE');
		INSERT INTO refresh_sessions(id, user_id, token_hash, token_family, expires_at, created_at)
		VALUES('session-1', 'user-1', 'hash', 'family', '2099-01-01T00:00:00Z', 'now');
	`, oldHash)
	if err != nil {
		t.Fatalf("seed database: %v", err)
	}

	service := NewPasswordService(repositories.NewUserRepository(db))
	service.now = func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) }
	if err = service.Change(context.Background(), "user-1", "wrong-password", newPassword, "127.0.0.1"); !errors.Is(err, ErrInvalidCurrentPassword) {
		t.Fatalf("wrong current password: %v", err)
	}
	if err = service.Change(context.Background(), "user-1", oldPassword, oldPassword, "127.0.0.1"); !errors.Is(err, ErrPasswordUnchanged) {
		t.Fatalf("unchanged password: %v", err)
	}
	if err = service.Change(context.Background(), "user-1", oldPassword, newPassword, "127.0.0.1"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	var newHash string
	var mustChange bool
	if err = db.QueryRow(`SELECT password_hash, must_change_password FROM users WHERE id='user-1'`).Scan(&newHash, &mustChange); err != nil {
		t.Fatalf("read new hash: %v", err)
	}
	valid, err := auth.VerifyPassword(newPassword, newHash)
	if err != nil || !valid {
		t.Fatalf("new password was not stored: valid=%t err=%v", valid, err)
	}
	if mustChange {
		t.Fatal("password change requirement was not cleared")
	}
	var revoked, audits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM refresh_sessions WHERE user_id='user-1' AND revoked_at IS NOT NULL`).Scan(&revoked)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE actor_user_id='user-1' AND action='PASSWORD_CHANGED'`).Scan(&audits)
	if revoked != 1 || audits != 1 {
		t.Fatalf("revoked=%d audits=%d", revoked, audits)
	}
}
