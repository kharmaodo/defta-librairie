package repositories

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func matchingDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "matching.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE defta (id INTEGER PRIMARY KEY, library_id TEXT, title TEXT, editeur TEXT, auteur TEXT, tags TEXT, categorie TEXT, deleted_at TEXT);
		CREATE VIRTUAL TABLE defta_fts USING fts5(title,editeur,auteur,tags,categorie,content='defta',content_rowid='id');
		CREATE TRIGGER defta_ai AFTER INSERT ON defta BEGIN
			INSERT INTO defta_fts(rowid,title,editeur,auteur,tags,categorie) VALUES(new.id,new.title,new.editeur,new.auteur,new.tags,new.categorie);
		END;
		CREATE TABLE cover_import_jobs (id TEXT PRIMARY KEY, library_id TEXT NOT NULL, actor_user_id TEXT NOT NULL,status TEXT NOT NULL,nsfw_decision TEXT,updated_at TEXT NOT NULL,failure_code TEXT,terminal_at TEXT);
		CREATE TABLE cover_import_ocr_results (job_id TEXT PRIMARY KEY,text_raw TEXT NOT NULL);
		CREATE TABLE cover_import_candidate_matches (job_id TEXT NOT NULL,book_id INTEGER NOT NULL,rank INTEGER NOT NULL,fts_score REAL,vector_score REAL,explanation_json TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(job_id,book_id),UNIQUE(job_id,rank));
		CREATE TABLE audit_logs (id TEXT PRIMARY KEY,actor_user_id TEXT NOT NULL,action TEXT NOT NULL,resource_type TEXT NOT NULL,resource_id TEXT NOT NULL,new_values TEXT NOT NULL,success INTEGER NOT NULL,created_at TEXT NOT NULL);
		CREATE TABLE cover_import_outbox (event_id TEXT PRIMARY KEY,event_type TEXT NOT NULL,payload TEXT NOT NULL,published_at TEXT,available_at TEXT NOT NULL,created_at TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,last_error TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func matchingSeed(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO defta(id,library_id,title) VALUES
		(1,'library-a','كتاب البداية'),(2,'library-b','كتاب البداية'),
		(3,'library-a','كتاب البداية'),(4,'library-a','كتاب البداية'),
		(5,'library-a','كتاب البداية'),(6,'library-a','كتاب البداية'),
		(7,'library-a','كتاب البداية'),(8,'library-a','كتاب البداية');
		UPDATE defta SET deleted_at='yesterday' WHERE id=8;
		INSERT INTO cover_import_jobs(id,library_id,actor_user_id,status,nsfw_decision,updated_at)
		VALUES('job-1','library-a','owner-a','MATCHING','SAFE','before');
		INSERT INTO cover_import_ocr_results(job_id,text_raw) VALUES('job-1','كتاب البداية');`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMatchingCandidatesAreScopedAndIdempotent(t *testing.T) {
	db := matchingDB(t)
	matchingSeed(t, db)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	if _, err := repo.MatchingJob(ctx, "job-1", "library-b"); !errors.Is(err, ErrCoverImportJobNotFound) {
		t.Fatalf("foreign library: %v", err)
	}
	job, err := repo.MatchingJob(ctx, "job-1", "library-a")
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := repo.SearchCoverImportCandidates(ctx, job.LibraryID, `"كتاب" OR "البداية"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 5 {
		t.Fatalf("expected top five, got %d", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.BookID == 2 || candidate.BookID == 8 {
			t.Fatalf("foreign or deleted book %d", candidate.BookID)
		}
	}
	if err = repo.CompleteMatching(ctx, job, candidates, "audit-1", "now"); err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteMatching(ctx, job, candidates, "audit-2", "again"); !errors.Is(err, ErrCoverImportJobState) {
		t.Fatalf("duplicate delivery: %v", err)
	}
	var candidateCount, auditCount, bookCount int
	for _, test := range []struct {
		query  string
		result *int
	}{
		{`SELECT COUNT(*) FROM cover_import_candidate_matches WHERE job_id='job-1'`, &candidateCount},
		{`SELECT COUNT(*) FROM audit_logs WHERE resource_id='job-1'`, &auditCount},
		{`SELECT COUNT(*) FROM defta`, &bookCount},
	} {
		if err = db.QueryRow(test.query).Scan(test.result); err != nil {
			t.Fatal(err)
		}
	}
	if candidateCount != 5 || auditCount != 1 || bookCount != 8 {
		t.Fatalf("candidates=%d audit=%d books=%d", candidateCount, auditCount, bookCount)
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM cover_import_jobs WHERE id='job-1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "REVIEW_REQUIRED" {
		t.Fatalf("status=%s", status)
	}
}

func TestMatchingEmptyCandidatesRequiresHumanReview(t *testing.T) {
	db := matchingDB(t)
	matchingSeed(t, db)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	job, err := repo.MatchingJob(ctx, "job-1", "library-a")
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := repo.SearchCoverImportCandidates(ctx, "library-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("unexpected candidates: %v", candidates)
	}
	if err = repo.CompleteMatching(ctx, job, candidates, "audit-empty", "now"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM cover_import_jobs WHERE id='job-1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "REVIEW_REQUIRED" {
		t.Fatalf("status=%s", status)
	}
}

func TestMatchingRejectsForeignCandidateAtomically(t *testing.T) {
	db := matchingDB(t)
	matchingSeed(t, db)
	repo := NewCoverImportRepository(db)
	job, err := repo.MatchingJob(context.Background(), "job-1", "library-a")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteMatching(context.Background(), job, []CoverImportCandidate{{BookID: 2, Score: 1}}, "audit-foreign", "now"); !errors.Is(err, ErrInvalidCoverImport) {
		t.Fatalf("foreign candidate: %v", err)
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM cover_import_jobs WHERE id='job-1'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "MATCHING" {
		t.Fatalf("partial transaction: %s", status)
	}
}
