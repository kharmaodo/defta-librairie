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
			moderation_score REAL,
			moderation_model_version TEXT,
			decision_code TEXT,
			created_book_id INTEGER,
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
		CREATE TABLE defta (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL, auteur TEXT, editeur TEXT, price REAL NOT NULL,
			volume INTEGER NOT NULL, status TEXT, tags TEXT, categorie TEXT, coverUrl TEXT,
			library_id TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			version INTEGER NOT NULL
		);
		CREATE TABLE library_settings (library_id TEXT PRIMARY KEY, default_low_stock_threshold INTEGER NOT NULL);
		CREATE TABLE book_inventory (
			book_id INTEGER PRIMARY KEY, library_id TEXT NOT NULL, quantity INTEGER NOT NULL,
			low_stock_threshold INTEGER NOT NULL, version INTEGER NOT NULL, updated_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestBookSubmissionManualReviewApproveCreatesOneBookAndAuditsReviewer(t *testing.T) {
	db := openBookSubmissionTestDB(t)
	if _, err := db.Exec(`
		INSERT INTO book_submissions(
			id, library_id, actor_user_id, title, auteur, editeur, price, volume, status,
			tags, categorie, cover_url, source_object_key, source_content_type, source_format,
			source_width, source_height, source_size, moderation_status, expires_at, created_at, updated_at
		) VALUES ('review-1','library-1','owner-1','Livre ambigu','Auteur','Editeur',2500,3,'AVAILABLE',
			'tag','Essai','','quarantine/library-1/review-1/source.jpg','image/jpeg','jpeg',800,1200,2048,
			'REVIEW_REQUIRED','2026-10-01T10:00:00Z','2026-09-23T10:00:00Z','2026-09-23T10:00:00Z')
	`); err != nil {
		t.Fatalf("seed review: %v", err)
	}
	repository := NewBookSubmissionRepository(db)
	bookID, err := repository.DecideReview(context.Background(), "review-1", "root-1", ManualReviewApprove,
		"audit-decision-1", "audit-book-1", "2026-09-23T10:01:00Z")
	if err != nil || bookID < 1 {
		t.Fatalf("approve review: id=%d err=%v", bookID, err)
	}
	if _, err = repository.DecideReview(context.Background(), "review-1", "root-1", ManualReviewApprove,
		"audit-decision-2", "audit-book-2", "2026-09-23T10:02:00Z"); !errors.Is(err, ErrBookSubmissionState) {
		t.Fatalf("second approval error=%v", err)
	}
	var status, code string
	var createdID int
	if err = db.QueryRow(`SELECT moderation_status, decision_code, created_book_id FROM book_submissions WHERE id='review-1'`).Scan(&status, &code, &createdID); err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if status != "APPROVED" || code != "MANUAL_APPROVED" || createdID != bookID {
		t.Fatalf("status=%s code=%s book=%d", status, code, createdID)
	}
	var books, audit int
	_ = db.QueryRow(`SELECT COUNT(*) FROM defta`).Scan(&books)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE actor_user_id='root-1' AND action='MANUALLY_DECIDE_BOOK_SUBMISSION_MODERATION'`).Scan(&audit)
	if books != 1 || audit != 1 {
		t.Fatalf("books=%d audit=%d", books, audit)
	}
}

func TestBookSubmissionManualReviewRejectDoesNotCreateBook(t *testing.T) {
	db := openBookSubmissionTestDB(t)
	if _, err := db.Exec(`
		INSERT INTO book_submissions(
			id, library_id, actor_user_id, title, auteur, editeur, price, volume, status,
			tags, categorie, cover_url, source_object_key, source_content_type, source_format,
			source_width, source_height, source_size, moderation_status, expires_at, created_at, updated_at
		) VALUES ('review-2','library-1','owner-1','Livre ambigu','Auteur','Editeur',2500,3,'AVAILABLE',
			'tag','Essai','','quarantine/library-1/review-2/source.jpg','image/jpeg','jpeg',800,1200,2048,
			'REVIEW_REQUIRED','2026-10-01T10:00:00Z','2026-09-23T10:00:00Z','2026-09-23T10:00:00Z')
	`); err != nil { t.Fatalf("seed review: %v", err) }
	if _, err := NewBookSubmissionRepository(db).DecideReview(context.Background(), "review-2", "root-1", ManualReviewReject,
		"audit-decision-2", "", "2026-09-23T10:01:00Z"); err != nil { t.Fatalf("reject review: %v", err) }
	var status string
	var books int
	_ = db.QueryRow(`SELECT moderation_status FROM book_submissions WHERE id='review-2'`).Scan(&status)
	_ = db.QueryRow(`SELECT COUNT(*) FROM defta`).Scan(&books)
	if status != "REJECTED" || books != 0 { t.Fatalf("status=%s books=%d", status, books) }
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

func TestBookSubmissionRetryFailedRequeuesExactlyOnce(t *testing.T) {
	db := openBookSubmissionTestDB(t)
	repository := NewBookSubmissionRepository(db)
	submission := PendingBookSubmission{ID: "retry-1", LibraryID: "library-1", ActorUserID: "owner-1", Book: models.BookInput{Title: "Livre", Price: 1, Volume: 0, Status: "AVAILABLE"}, SourceObjectKey: "quarantine/library-1/retry-1/source.jpg", SourceContentType: "image/jpeg", SourceFormat: "jpeg", SourceWidth: 1, SourceHeight: 1, SourceSize: 1, ExpiresAt: "2026-10-01T10:00:00Z"}
	if err := repository.CreatePending(context.Background(), submission, "event-initial", `{"submissionId":"retry-1"}`, "audit-initial", "2026-09-23T10:00:00Z"); err != nil { t.Fatal(err) }
	if _, err := db.Exec(`UPDATE book_submissions SET moderation_status='FAILED', decision_code='MODERATOR_UNAVAILABLE' WHERE id='retry-1'`); err != nil { t.Fatal(err) }
	if err := repository.RetryFailed(context.Background(), "retry-1", "root-1", "event-retry", `{"submissionId":"retry-1"}`, "audit-retry", "2026-09-23T10:01:00Z"); err != nil { t.Fatal(err) }
	if err := repository.RetryFailed(context.Background(), "retry-1", "root-1", "event-retry-2", `{"submissionId":"retry-1"}`, "audit-retry-2", "2026-09-23T10:02:00Z"); !errors.Is(err, ErrBookSubmissionState) { t.Fatalf("second retry: %v", err) }
	var status, code string; var events, audits int
	_ = db.QueryRow(`SELECT moderation_status, decision_code FROM book_submissions WHERE id='retry-1'`).Scan(&status, &code)
	_ = db.QueryRow(`SELECT COUNT(*) FROM book_submission_outbox WHERE submission_id='retry-1'`).Scan(&events)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='RETRY_BOOK_SUBMISSION_MODERATION'`).Scan(&audits)
	if status != "PENDING_SCAN" || code != "RETRY_REQUESTED" || events != 2 || audits != 1 { t.Fatalf("status=%s code=%s events=%d audits=%d", status, code, events, audits) }
}
