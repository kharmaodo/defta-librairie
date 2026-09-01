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

	_ "github.com/mattn/go-sqlite3"
)

func TestBookLifecycleAndLibraryIsolation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "books.db")+"?_foreign_keys=on")
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
		CREATE TABLE defta (
			id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, auteur TEXT, editeur TEXT,
			price REAL NOT NULL DEFAULT 0, volume INTEGER NOT NULL DEFAULT 0, status TEXT, tags TEXT,
			categorie TEXT, coverUrl TEXT, library_id TEXT, created_at TEXT, updated_at TEXT,
			deleted_at TEXT, version INTEGER NOT NULL DEFAULT 1
		);
		CREATE TABLE audit_logs (id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT,
		resource_type TEXT, resource_id TEXT, old_values TEXT, new_values TEXT, ip_address TEXT,
		success INTEGER, created_at TEXT);
		INSERT INTO users(id,username,password_hash,role,status,created_at,updated_at) VALUES
		('root','root','hash','SUPER_ADMIN_ROOT','ACTIVE','now','now'),
		('owner-1','owner1','hash','OWNER_LIBRARY','ACTIVE','now','now'),
		('owner-2','owner2','hash','OWNER_LIBRARY','ACTIVE','now','now');
		INSERT INTO libraries(id,name,owner_user_id,status,created_at,updated_at) VALUES
		('library-1','One','owner-1','ACTIVE','now','now'),
		('library-2','Two','owner-2','ACTIVE','now','now');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	service := NewBookService(repositories.NewBookRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	root.Subject = "root"

	book, err := service.Create(context.Background(), ownerOne, models.BookInput{
		Title: "Livre Un", Price: 2500, Volume: 1, Tags: "fiqh,arabe",
	})
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	if book.LibraryID != "library-1" || book.Version != 1 {
		t.Fatalf("unexpected book: %+v", book)
	}
	if _, err = service.Find(context.Background(), ownerTwo, book.ID); !errors.Is(err, repositories.ErrBookNotFound) {
		t.Fatalf("cross-library read must be hidden, got %v", err)
	}

	updated, err := service.Update(context.Background(), ownerOne, book.ID, models.BookInput{
		Title: "Livre Un modifié", Price: 3000, Volume: book.Volume, Tags: "fiqh,édition",
		LibraryID: book.LibraryID, Version: book.Version,
	})
	if err != nil {
		t.Fatalf("update book: %v", err)
	}
	if updated.Version != 2 || updated.Price != 3000 {
		t.Fatalf("unexpected updated book: %+v", updated)
	}
	_, err = service.Update(context.Background(), ownerOne, book.ID, models.BookInput{
		Title: "Stale", Price: 1, Version: 1,
	})
	if !errors.Is(err, repositories.ErrBookConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}

	rootBooks, total, err := service.List(context.Background(), root, "", 0, 30)
	if err != nil || total != 1 || len(rootBooks) != 1 {
		t.Fatalf("root list: total=%d len=%d err=%v", total, len(rootBooks), err)
	}
	if err = service.Delete(context.Background(), ownerOne, book.ID); err != nil {
		t.Fatalf("delete book: %v", err)
	}
	if _, err = service.Find(context.Background(), root, book.ID); !errors.Is(err, repositories.ErrBookNotFound) {
		t.Fatalf("deleted book must be hidden, got %v", err)
	}
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='BOOK'`).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}

func TestRootMustChooseLibraryWhenCreatingBook(t *testing.T) {
	service := NewBookService(nil)
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	root.Subject = "root"
	_, err := service.Create(context.Background(), root, models.BookInput{Title: "Livre", Price: 1})
	if !errors.Is(err, ErrInvalidBook) {
		t.Fatalf("expected ErrInvalidBook, got %v", err)
	}
}
