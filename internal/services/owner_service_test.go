package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestOwnerLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "owners.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE, email TEXT UNIQUE COLLATE NOCASE,
			password_hash TEXT NOT NULL, role TEXT NOT NULL, status TEXT NOT NULL,
			failed_login_attempts INTEGER NOT NULL DEFAULT 0, locked_until TEXT, last_login_at TEXT,
			password_changed_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE TABLE libraries (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT, owner_user_id TEXT UNIQUE,
			status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			FOREIGN KEY(owner_user_id) REFERENCES users(id)
		);
		CREATE TABLE refresh_sessions (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			token_family TEXT NOT NULL, expires_at TEXT NOT NULL, revoked_at TEXT, replaced_by_id TEXT,
			ip_address TEXT, user_agent TEXT, created_at TEXT NOT NULL, last_used_at TEXT
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT, old_values TEXT, new_values TEXT, ip_address TEXT,
			success INTEGER NOT NULL, created_at TEXT NOT NULL
		);
		INSERT INTO users(id, username, password_hash, role, status, created_at, updated_at)
		VALUES('root', 'root-admin', 'hash', 'SUPER_ADMIN_ROOT', 'ACTIVE', 'now', 'now');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	service := NewOwnerService(repositories.NewOwnerRepository(db))
	owner, err := service.Create(context.Background(), models.OwnerCreateInput{
		Username: "owner-one", Email: "owner@example.com", Password: "Correct-Horse-2026",
		LibraryName: "Librairie Une", LibraryDescription: "Première librairie",
	}, "root")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if owner.Library.ID == "" || owner.Status != models.UserStatusActive {
		t.Fatalf("unexpected owner: %+v", owner)
	}
	_, err = db.Exec(`
		UPDATE users SET status='LOCKED', failed_login_attempts=5, locked_until='2099-01-01T00:00:00Z' WHERE id=?;
		INSERT INTO refresh_sessions(id, user_id, token_hash, token_family, expires_at, created_at)
		VALUES('locked-session', ?, 'locked-hash', 'locked-family', '2099-01-01T00:00:00Z', 'now');
	`, owner.ID, owner.ID)
	if err != nil {
		t.Fatalf("lock owner: %v", err)
	}
	if err = service.Unlock(context.Background(), owner.ID, "root"); err != nil {
		t.Fatalf("unlock owner: %v", err)
	}
	var attempts int
	var lockedUntil, revokedAt sql.NullString
	if err = db.QueryRow(`
		SELECT u.failed_login_attempts, u.locked_until, s.revoked_at
		FROM users u JOIN refresh_sessions s ON s.user_id=u.id WHERE u.id=?
	`, owner.ID).Scan(&attempts, &lockedUntil, &revokedAt); err != nil {
		t.Fatalf("read unlocked owner: %v", err)
	}
	if attempts != 0 || lockedUntil.Valid || !revokedAt.Valid {
		t.Fatalf("attempts=%d lockedUntil=%v revokedAt=%v", attempts, lockedUntil, revokedAt)
	}
	if err = service.Unlock(context.Background(), owner.ID, "root"); !errors.Is(err, repositories.ErrOwnerNotLocked) {
		t.Fatalf("unlock active owner error=%v", err)
	}

	newName := "Librairie Modifiée"
	disabled := models.UserStatusDisabled
	updated, err := service.Update(context.Background(), owner.ID, models.OwnerUpdateInput{
		Status: &disabled, LibraryName: &newName,
	}, "root")
	if err != nil {
		t.Fatalf("update owner: %v", err)
	}
	if updated.Status != disabled || updated.Library.Name != newName {
		t.Fatalf("unexpected updated owner: %+v", updated)
	}

	owners, err := service.List(context.Background())
	if err != nil || len(owners) != 1 {
		t.Fatalf("list owners: len=%d err=%v", len(owners), err)
	}
	filtered, total, err := service.Search(context.Background(), "modifiée", "disabled", "disabled", 0, 10)
	if err != nil || total != 1 || len(filtered) != 1 || filtered[0].ID != owner.ID {
		t.Fatalf("filtered owners: total=%d owners=%+v err=%v", total, filtered, err)
	}
	empty, total, err := service.Search(context.Background(), "introuvable", "", "", 0, 10)
	if err != nil || total != 0 || len(empty) != 0 {
		t.Fatalf("empty owner search: total=%d owners=%+v err=%v", total, empty, err)
	}
	if _, _, err = service.Search(context.Background(), "", "unknown", "", 0, 10); err != ErrInvalidOwner {
		t.Fatalf("expected invalid filter, got %v", err)
	}
	if err = service.Disable(context.Background(), owner.ID, "root"); err != nil {
		t.Fatalf("disable owner: %v", err)
	}

	var userStatus, libraryStatus string
	if err = db.QueryRow(`
		SELECT u.status, l.status FROM users u JOIN libraries l ON l.owner_user_id=u.id WHERE u.id=?
	`, owner.ID).Scan(&userStatus, &libraryStatus); err != nil {
		t.Fatalf("read disabled state: %v", err)
	}
	if userStatus != "DISABLED" || libraryStatus != "DISABLED" {
		t.Fatalf("user=%s library=%s", userStatus, libraryStatus)
	}
	if err = service.Reactivate(context.Background(), owner.ID, "root"); err != nil {
		t.Fatalf("reactivate owner: %v", err)
	}
	if err = db.QueryRow(`
		SELECT u.status, l.status FROM users u JOIN libraries l ON l.owner_user_id=u.id WHERE u.id=?
	`, owner.ID).Scan(&userStatus, &libraryStatus); err != nil {
		t.Fatalf("read reactivated state: %v", err)
	}
	if userStatus != "ACTIVE" || libraryStatus != "ACTIVE" {
		t.Fatalf("reactivated user=%s library=%s", userStatus, libraryStatus)
	}
	if err = service.Reactivate(context.Background(), owner.ID, "root"); !errors.Is(err, repositories.ErrOwnerNotDisabled) {
		t.Fatalf("reactivate active owner error=%v", err)
	}
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE actor_user_id='root'`).Scan(&audits); err != nil || audits != 5 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}

func TestCreateOwnerRejectsInvalidInput(t *testing.T) {
	service := NewOwnerService(nil)
	_, err := service.Create(context.Background(), models.OwnerCreateInput{
		Username: "x", Password: "short", LibraryName: "L",
	}, "root")
	if err != ErrInvalidOwner {
		t.Fatalf("expected ErrInvalidOwner, got %v", err)
	}
}
