package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openBookSubmissionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "book-submissions.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`
		CREATE TABLE book_submissions (
			id TEXT PRIMARY KEY,
			library_id TEXT NOT NULL,
			actor_user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			auteur TEXT NOT NULL,
			editeur TEXT NOT NULL,
			price REAL NOT NULL,
			volume INTEGER NOT NULL,
			status TEXT NOT NULL,
			tags TEXT NOT NULL,
			categorie TEXT NOT NULL,
			cover_url TEXT NOT NULL,
			source_object_key TEXT NOT NULL UNIQUE,
			source_content_type TEXT NOT NULL,
			source_format TEXT NOT NULL,
			source_width INTEGER NOT NULL,
			source_height INTEGER NOT NULL,
			source_size INTEGER NOT NULL,
			moderation_status TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE book_submission_outbox (
			event_id TEXT PRIMARY KEY,
			submission_id TEXT NOT NULL,
			library_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			payload TEXT NOT NULL,
			attempts INTEGER NOT NULL,
			available_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY,
			actor_user_id TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			new_values TEXT NOT NULL,
			success INTEGER NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestBookSubmissionCreatePendingIsAtomic(t *testing.T) {
	db := openBookSubmissionTestDB(t)
	repository := NewBookSubmissionRepository(db)
	submission := PendingBookSubmission{
		ID: "submission-1", LibraryID: "library-1", ActorUserID: "owner-1",
		Book: models.BookInput{
			Title: "Livre en quarantaine", Auteur: "Auteur", Editeur: "Editeur",
			Price: 2500, Volume: 3, Status: "AVAILABLE", Tags: "tag", Categorie: "Essai",
		},
		SourceObjectKey: "quarantine/library-1/submission-1/source.jpg",
		SourceContentType: "image/jpeg", SourceFormat: "jpeg",
		SourceWidth: 800, SourceHeight: 1200, SourceSize: 2048,
		ExpiresAt: "2026-10-01T10:00:00Z",
	}
	payload := `{"schemaVersion":1,"submissionId":"submission-1"}`
	if err := repository.CreatePending(
		context.Background(), submission, "event-1", payload, "audit-1",
		"2026-09-23T10:00:00Z",
	); err != nil {
		t.Fatalf("create pending: %v", err)
	}

	var status string
	if err := db.QueryRow(
		"SELECT moderation_status FROM book_submissions WHERE id='submission-1'",
	).Scan(&status); err != nil {
		t.Fatalf("read submission: %v", err)
	}
	if status != "PENDING_SCAN" {
		t.Fatalf("status=%q", status)
	}

	var outbox, audit int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM book_submission_outbox WHERE submission_id='submission-1'",
	).Scan(&outbox); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM audit_logs WHERE action='CREATE_BOOK_SUBMISSION'",
	).Scan(&audit); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if outbox != 1 || audit != 1 {
		t.Fatalf("outbox=%d audit=%d", outbox, audit)
	}
}

func TestBookSubmissionCreatePendingRejectsIncompleteSubmission(t *testing.T) {
	repository := NewBookSubmissionRepository(openBookSubmissionTestDB(t))
	err := repository.CreatePending(
		context.Background(),
		PendingBookSubmission{ID: "missing-source", LibraryID: "library-1", ActorUserID: "owner-1"},
		"event-1", "{}", "audit-1", "2026-09-23T10:00:00Z",
	)
	if !errors.Is(err, ErrInvalidBookSubmission) {
		t.Fatalf("error=%v", err)
	}
}
