package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"path/filepath"
	"testing"
)

func TestLibrarySettingsLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "settings.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialAcceptanceFixture); err != nil {
		t.Fatal(err)
	}
	s := NewLibrarySettingsService(repositories.NewLibrarySettingsRepository(db))
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "accept-library", RegisteredClaims: jwt.RegisteredClaims{Subject: "accept-owner"}}
	v, err := s.Find(ctx, owner, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Currency != "XOF" || v.DefaultLowStockThreshold != 5 || v.Version != 1 {
		t.Fatalf("defaults=%+v", v)
	}
	if _, err = s.Find(ctx, owner, "other"); !errors.Is(err, ErrBookForbidden) {
		t.Fatalf("scope=%v", err)
	}
	if _, err = s.Find(ctx, nil, ""); !errors.Is(err, ErrBookForbidden) {
		t.Fatalf("anonymous=%v", err)
	}
	for _, change := range []func(*models.LibrarySettings){func(v *models.LibrarySettings) { v.Currency = "EUR" }, func(v *models.LibrarySettings) { v.DefaultLowStockThreshold = -1 }, func(v *models.LibrarySettings) { v.Email = "invalid" }, func(v *models.LibrarySettings) { v.LogoData = "data:image/svg+xml;base64,PHN2Zz4=" }, func(v *models.LibrarySettings) { v.LogoData = "data:image/png;base64,YmFk" }} {
		bad := v
		change(&bad)
		if err = s.Update(ctx, owner, "", bad); !errors.Is(err, ErrInvalidSettings) {
			t.Fatalf("validation=%v", err)
		}
	}
	v.Address = "Dakar"
	v.PrintFooter = "شكرا"
	v.DefaultLowStockThreshold = 9
	if err = s.Update(ctx, owner, "", v); err != nil {
		t.Fatal(err)
	}
	if err = s.Update(ctx, owner, "", v); !errors.Is(err, repositories.ErrSettingsConflict) {
		t.Fatalf("stale update=%v", err)
	}
	got, err := s.Find(ctx, owner, "")
	if err != nil || got.Version != 2 || got.PrintFooter != "شكرا" {
		t.Fatalf("read=%+v err=%v", got, err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='UPDATE_LIBRARY_SETTINGS'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit=%d err=%v", count, err)
	}
	book, err := repositories.NewBookRepository(db).Create(ctx, models.BookInput{Title: "New book", LibraryID: "accept-library", Status: "AVAILABLE", Price: 100}, "accept-owner", "new-book-audit", "{}", "2026-09-09T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	var threshold int
	if err = db.QueryRow("SELECT low_stock_threshold FROM book_inventory WHERE book_id=?", book.ID).Scan(&threshold); err != nil || threshold != 9 {
		t.Fatalf("new threshold=%d err=%v", threshold, err)
	}
	if err = db.QueryRow("SELECT low_stock_threshold FROM book_inventory WHERE book_id=1").Scan(&threshold); err != nil || threshold != 5 {
		t.Fatalf("existing threshold=%d err=%v", threshold, err)
	}
	if _, err = db.Exec("UPDATE libraries SET status='DISABLED' WHERE id='accept-library'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Find(ctx, owner, ""); !errors.Is(err, repositories.ErrSettingsNotFound) {
		t.Fatalf("disabled library=%v", err)
	}
}
