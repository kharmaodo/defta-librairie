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

	_ "github.com/mattn/go-sqlite3"
)

func TestRefreshRotationReuseDetectionAndLogout(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "sessions.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT, email TEXT, password_hash TEXT,
		role TEXT, status TEXT, failed_login_attempts INTEGER DEFAULT 0, locked_until TEXT,
		last_login_at TEXT, password_changed_at TEXT, created_at TEXT, updated_at TEXT);
		CREATE TABLE libraries (id TEXT PRIMARY KEY, name TEXT, description TEXT, owner_user_id TEXT,
		status TEXT, created_at TEXT, updated_at TEXT);
		CREATE TABLE refresh_sessions (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE,
			token_family TEXT NOT NULL, expires_at TEXT NOT NULL, revoked_at TEXT,
			replaced_by_id TEXT, ip_address TEXT, user_agent TEXT, created_at TEXT NOT NULL,
			last_used_at TEXT, FOREIGN KEY(user_id) REFERENCES users(id),
			FOREIGN KEY(replaced_by_id) REFERENCES refresh_sessions(id)
		);
		CREATE TABLE audit_logs (id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT,
		resource_type TEXT, resource_id TEXT, old_values TEXT, new_values TEXT,
		ip_address TEXT, success INTEGER, created_at TEXT);
		INSERT INTO users(id,username,password_hash,role,status,created_at,updated_at)
		VALUES('root','root','hash','SUPER_ADMIN_ROOT','ACTIVE','now','now');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	tokens, err := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", 15*time.Minute)
	if err != nil {
		t.Fatalf("token manager: %v", err)
	}
	service, err := NewSessionService(repositories.NewSessionRepository(db), tokens, 24*time.Hour)
	if err != nil {
		t.Fatalf("session service: %v", err)
	}
	user := models.User{ID: "root", Username: "root", Role: models.RoleSuperAdminRoot, Status: models.UserStatusActive}
	started, err := service.Start(context.Background(), user, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if started.AccessToken == "" || started.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	claims, err := tokens.Parse(started.AccessToken)
	if err != nil || claims.SessionID == "" {
		t.Fatalf("access token session claim: claims=%+v err=%v", claims, err)
	}
	active, err := repositories.NewSessionRepository(db).IsActive(context.Background(), claims.SessionID,
		claims.Subject, claims.Role, claims.LibraryID, time.Now())
	if err != nil || !active {
		t.Fatalf("started session must be active: active=%v err=%v", active, err)
	}
	var storedHash string
	if err = db.QueryRow(`SELECT token_hash FROM refresh_sessions`).Scan(&storedHash); err != nil {
		t.Fatalf("read token hash: %v", err)
	}
	if storedHash == started.RefreshToken || storedHash != auth.HashRefreshToken(started.RefreshToken) {
		t.Fatal("raw refresh token must not be stored")
	}

	rotated, err := service.Refresh(context.Background(), started.RefreshToken, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("rotate session: %v", err)
	}
	if rotated.RefreshToken == started.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	active, err = repositories.NewSessionRepository(db).IsActive(context.Background(), claims.SessionID,
		claims.Subject, claims.Role, claims.LibraryID, time.Now())
	if err != nil || active {
		t.Fatalf("rotated access session must be inactive: active=%v err=%v", active, err)
	}
	rotatedClaims, err := tokens.Parse(rotated.AccessToken)
	if err != nil {
		t.Fatalf("parse rotated access token: %v", err)
	}
	active, err = repositories.NewSessionRepository(db).IsActive(context.Background(), rotatedClaims.SessionID,
		rotatedClaims.Subject, rotatedClaims.Role, rotatedClaims.LibraryID, time.Now())
	if err != nil || !active {
		t.Fatalf("replacement access session must be active: active=%v err=%v", active, err)
	}
	if _, err = service.Refresh(context.Background(), started.RefreshToken, "127.0.0.1", "test"); !errors.Is(err, ErrRefreshTokenReuse) {
		t.Fatalf("expected reuse detection, got %v", err)
	}
	if _, err = service.Refresh(context.Background(), rotated.RefreshToken, "127.0.0.1", "test"); !errors.Is(err, ErrRefreshTokenReuse) {
		t.Fatalf("reused family must remain revoked, got %v", err)
	}

	second, err := service.Start(context.Background(), user, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("start second session: %v", err)
	}
	if err = service.Logout(context.Background(), second.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	secondClaims, _ := tokens.Parse(second.AccessToken)
	active, err = repositories.NewSessionRepository(db).IsActive(context.Background(), secondClaims.SessionID,
		secondClaims.Subject, secondClaims.Role, secondClaims.LibraryID, time.Now())
	if err != nil || active {
		t.Fatalf("logged-out access session must be inactive: active=%v err=%v", active, err)
	}
	if _, err = service.Refresh(context.Background(), second.RefreshToken, "127.0.0.1", "test"); !errors.Is(err, ErrRefreshTokenReuse) {
		t.Fatalf("logged-out token must be rejected, got %v", err)
	}

	var reuseAudits, logoutAudits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='REFRESH_TOKEN_REUSE'`).Scan(&reuseAudits)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='LOGOUT'`).Scan(&logoutAudits)
	if reuseAudits < 1 || logoutAudits != 1 {
		t.Fatalf("reuse audits=%d logout audits=%d", reuseAudits, logoutAudits)
	}
}
