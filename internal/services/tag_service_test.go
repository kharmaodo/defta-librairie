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

func TestLibraryTagLifecycleAndIsolation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "tags.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (id TEXT PRIMARY KEY, username TEXT, role TEXT);
		CREATE TABLE libraries (id TEXT PRIMARY KEY, name TEXT, owner_user_id TEXT, status TEXT);
		CREATE TABLE library_tags (
			id TEXT PRIMARY KEY, library_id TEXT NOT NULL, name TEXT NOT NULL,
			normalized_name TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(library_id, normalized_name), FOREIGN KEY(library_id) REFERENCES libraries(id)
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT, resource_type TEXT,
			resource_id TEXT, old_values TEXT, new_values TEXT, success INTEGER, created_at TEXT
		);
		INSERT INTO users VALUES ('root','root','SUPER_ADMIN_ROOT'), ('owner-1','one','OWNER_LIBRARY'), ('owner-2','two','OWNER_LIBRARY');
		INSERT INTO libraries VALUES ('library-1','One','owner-1','ACTIVE'), ('library-2','Two','owner-2','ACTIVE');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	service := NewTagService(repositories.NewTagRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	root.Subject = "root"

	tag, err := service.Create(context.Background(), ownerOne, "  Sciences   islamiques ", "")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.Name != "Sciences islamiques" || tag.LibraryID != "library-1" {
		t.Fatalf("unexpected tag: %+v", tag)
	}
	if _, err = service.Create(context.Background(), ownerOne, "sciences islamiques", ""); !errors.Is(err, repositories.ErrTagConflict) {
		t.Fatalf("expected duplicate conflict, got %v", err)
	}
	if _, err = service.Update(context.Background(), ownerTwo, tag.ID, "Interdit"); !errors.Is(err, repositories.ErrTagNotFound) {
		t.Fatalf("cross-library update must be hidden, got %v", err)
	}
	updated, err := service.Update(context.Background(), ownerOne, tag.ID, "Fiqh")
	if err != nil || updated.Name != "Fiqh" {
		t.Fatalf("update tag: tag=%+v err=%v", updated, err)
	}
	if _, err = service.Create(context.Background(), root, "Arabe", "library-2"); err != nil {
		t.Fatalf("root create tag: %v", err)
	}
	all, err := service.List(context.Background(), root, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("root list: len=%d err=%v", len(all), err)
	}
	ownerTags, err := service.List(context.Background(), ownerOne, "")
	if err != nil || len(ownerTags) != 1 || ownerTags[0].LibraryID != "library-1" {
		t.Fatalf("owner list: tags=%+v err=%v", ownerTags, err)
	}
	if err = service.Delete(context.Background(), ownerOne, tag.ID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='TAG'`).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}

func TestTagValidation(t *testing.T) {
	if _, _, err := normalizeTag("x"); !errors.Is(err, ErrInvalidTag) {
		t.Fatalf("expected short tag rejection, got %v", err)
	}
	if _, _, err := normalizeTag("a,b"); !errors.Is(err, ErrInvalidTag) {
		t.Fatalf("expected comma rejection, got %v", err)
	}
}
