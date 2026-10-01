package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestManualCoverSearchScopeDismissAndPromotion(t *testing.T) {
	db := reviewServiceDB(t)
	_, err := db.Exec(`CREATE VIRTUAL TABLE defta_fts USING fts5(title,auteur);
 INSERT INTO defta(id,library_id,title,auteur) VALUES(2,'library-a','Guide arabe','Auteur'),(3,'library-b','Guide arabe','Auteur'),(4,'library-a','Guide arabe','Auteur'),(5,'library-a','Guide arabe','Auteur'),(6,'library-a','Guide arabe','Auteur');
 INSERT INTO defta_fts(rowid,title,auteur) SELECT id,title,auteur FROM defta;
 INSERT INTO book_covers(id,book_id,library_id,status,active) VALUES('old-cover',4,'library-a','READY',1);
 CREATE TABLE cover_import_ocr_results(job_id TEXT,isbn13 TEXT);
 INSERT INTO cover_import_ocr_results VALUES('previous','9781234567897');
 INSERT INTO cover_import_jobs(id,library_id,status,nsfw_decision,target_book_id) VALUES('previous','library-a','READY','SAFE',2);`)
	if err != nil {
		t.Fatal(err)
	}
	store := &reviewMemoryStore{}
	svc := NewCoverImportReviewService(NewBookService(repositories.NewBookRepository(db)), repositories.NewCoverImportRepository(db), store)
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-a", RegisteredClaims: jwt.RegisteredClaims{Subject: "owner"}}
	foreign := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-b", RegisteredClaims: jwt.RegisteredClaims{Subject: "other"}}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot, RegisteredClaims: jwt.RegisteredClaims{Subject: "root"}}
	ctx := context.Background()
	if _, err = svc.SearchCandidates(ctx, foreign, "safe", "", "Guide"); !errors.Is(err, repositories.ErrCoverImportJobNotFound) {
		t.Fatalf("foreign %v", err)
	}
	if _, err = svc.SearchCandidates(ctx, root, "safe", "library-b", "Guide"); !errors.Is(err, repositories.ErrCoverImportJobNotFound) {
		t.Fatalf("root scope %v", err)
	}
	if _, err = svc.SearchCandidates(ctx, owner, "quarantine", "", "Guide"); !errors.Is(err, repositories.ErrCoverImportReviewState) {
		t.Fatalf("unsafe %v", err)
	}
	if _, err = svc.SearchCandidates(ctx, owner, "safe", "", " "); !errors.Is(err, ErrInvalidBook) {
		t.Fatalf("blank %v", err)
	}
	results, err := svc.SearchCandidates(ctx, owner, "safe", "", "Guide")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].BookID != 2 || results[1].BookID != 4 || results[2].BookID != 5 {
		t.Fatalf("results %+v", results)
	}
	if !results[1].HasActiveCover {
		t.Fatal("existing cover flag missing")
	}
	isbn, err := svc.SearchCandidates(ctx, owner, "safe", "", "978-1234567897")
	if err != nil || len(isbn) != 1 || isbn[0].BookID != 2 {
		t.Fatalf("isbn %+v %v", isbn, err)
	}
	if err = svc.DismissCandidate(ctx, owner, "safe", "", 2); err != nil {
		t.Fatal(err)
	}
	if err = svc.DismissCandidate(ctx, owner, "safe", "", 2); err != nil {
		t.Fatal(err)
	}
	var status string
	db.QueryRow(`SELECT status FROM cover_import_jobs WHERE id='safe'`).Scan(&status)
	if status != "REVIEW_REQUIRED" {
		t.Fatalf("dismiss terminated image: %s", status)
	}
	var audits int
	db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='DISMISS_COVER_IMPORT_CANDIDATE'`).Scan(&audits)
	if audits != 1 {
		t.Fatalf("audit replay %d", audits)
	}
	results, err = svc.SearchCandidates(ctx, owner, "safe", "", "Guide")
	if err != nil || len(results) != 3 || results[0].BookID != 4 {
		t.Fatalf("dismissed results %+v %v", results, err)
	}
	if _, err = svc.Decide(ctx, owner, "safe", "", "ACCEPT", "", 2); !errors.Is(err, repositories.ErrCoverImportCandidateNotFound) {
		t.Fatalf("dismissed accept %v", err)
	}
	if _, err = svc.Decide(ctx, owner, "safe", "", "ACCEPT", "", 4); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Decide(ctx, owner, "safe", "", "ACCEPT", "", 4); !errors.Is(err, repositories.ErrCoverImportReviewState) {
		t.Fatalf("replay %v", err)
	}
	var oldActive int
	db.QueryRow(`SELECT active FROM book_covers WHERE id='old-cover'`).Scan(&oldActive)
	if oldActive != 1 {
		t.Fatal("old cover changed before processing")
	}
	if store.copied != 1 {
		t.Fatalf("duplicate copy %d", store.copied)
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM cover_processing_outbox`).Scan(&count)
	if count != 1 {
		t.Fatalf("outbox %d", count)
	}
	var method string
	db.QueryRow(`SELECT json_extract(new_values,'$.selectionMethod') FROM audit_logs WHERE action='DECIDE_COVER_IMPORT_REVIEW'`).Scan(&method)
	if method != "MANUAL" {
		t.Fatalf("audit method %s", method)
	}
}

func TestManualCoverSearchZeroOneAndDeletedSuggestions(t *testing.T) {
	db := reviewServiceDB(t)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE defta_fts USING fts5(title,auteur);
 INSERT INTO defta(id,library_id,title,auteur,deleted_at)VALUES(2,'library-a','Unique guide','Author',NULL),(3,'library-b','Unique guide','Author',NULL),(4,'library-a','Unique guide','Author','deleted');
 INSERT INTO defta_fts(rowid,title,auteur) SELECT id,title,auteur FROM defta;`); err != nil {
		t.Fatal(err)
	}
	store := &reviewMemoryStore{}
	service := NewCoverImportReviewService(NewBookService(repositories.NewBookRepository(db)), repositories.NewCoverImportRepository(db), store)
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-a", RegisteredClaims: jwt.RegisteredClaims{Subject: "owner"}}
	ctx := context.Background()
	for _, query := range []string{"inexistant", "NEAR OR NOT"} {
		result, err := service.SearchCandidates(ctx, owner, "safe", "", query)
		if err != nil || len(result) != 0 {
			t.Fatalf("zero/literal query: %v %v", result, err)
		}
	}
	result, err := service.SearchCandidates(ctx, owner, "safe", "", "Unique")
	if err != nil || len(result) != 1 || result[0].BookID != 2 {
		t.Fatalf("single eligible result: %v %v", result, err)
	}
	if _, err = db.Exec(`UPDATE defta SET deleted_at='after-search' WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Decide(ctx, owner, "safe", "", "ACCEPT", "", 2); !errors.Is(err, repositories.ErrCoverImportCandidateNotFound) {
		t.Fatalf("deleted suggestion accepted: %v", err)
	}
	if store.copied != 0 {
		t.Fatal("deleted suggestion copied source")
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM cover_import_jobs WHERE id='safe'`).Scan(&status); err != nil || status != "REVIEW_REQUIRED" {
		t.Fatalf("failed selection terminated job: %s %v", status, err)
	}
}

func TestManualCoverReplacementFailureKeepsExistingCover(t *testing.T) {
	imports, owner, db := importErrorFixture(t, &importErrorStore{})
	ctx := context.Background()
	if _, err := imports.Create(ctx, owner, "", "manual-replacement-final", []CoverImportFile{importPNGFile(t)}); err != nil {
		t.Fatal(err)
	}
	var jobID string
	if err := db.QueryRow(`SELECT id FROM cover_import_jobs`).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE cover_import_jobs SET status='REVIEW_REQUIRED',nsfw_decision='SAFE';
 INSERT INTO defta(id,library_id,title) VALUES(1,'library-a','Guide manuel');
 INSERT INTO book_covers(id,book_id,library_id,status,source_object_key,source_content_type,source_format,source_width,source_height,source_size,large_jpeg_object_key,active,created_at,updated_at)
 VALUES('existing-cover',1,'library-a','READY','old-private-source','image/png','png',1,1,1,'old-private-preview',1,'before','before');`); err != nil {
		t.Fatal(err)
	}
	store := &reviewMemoryStore{}
	service := NewCoverImportReviewService(NewBookService(repositories.NewBookRepository(db)), repositories.NewCoverImportRepository(db), store)
	suggestions, err := service.SearchCandidates(ctx, owner, jobID, "", "Guide manuel")
	if err != nil || len(suggestions) != 1 || !suggestions[0].HasActiveCover {
		t.Fatalf("replacement not advertised: %v %v", suggestions, err)
	}
	if _, err = service.Decide(ctx, owner, jobID, "", "ACCEPT", "", 1); err != nil {
		t.Fatal(err)
	}
	var newCoverID string
	if err = db.QueryRow(`SELECT cover_id FROM cover_processing_outbox`).Scan(&newCoverID); err != nil {
		t.Fatal(err)
	}
	if err = repositories.NewCoverProcessingRepository(db).MarkFailed(ctx, newCoverID, "MAX_DELIVERIES", time.Now()); err != nil {
		t.Fatal(err)
	}
	var oldActive, newActive int
	var oldStatus, newStatus, preview string
	if err = db.QueryRow(`SELECT status,active,large_jpeg_object_key FROM book_covers WHERE id='existing-cover'`).Scan(&oldStatus, &oldActive, &preview); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT status,active FROM book_covers WHERE id=?`, newCoverID).Scan(&newStatus, &newActive); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "READY" || oldActive != 1 || preview != "old-private-preview" || newStatus != "FAILED" || newActive != 0 {
		t.Fatal("failed derivative replaced the existing cover")
	}
	if _, err = service.Decide(ctx, owner, jobID, "", "ACCEPT", "", 1); !errors.Is(err, repositories.ErrCoverImportReviewState) {
		t.Fatalf("replayed decision: %v", err)
	}
	for _, query := range []string{`SELECT COUNT(*) FROM cover_processing_outbox`, `SELECT COUNT(*) FROM audit_logs WHERE action='DECIDE_COVER_IMPORT_REVIEW'`} {
		var count int
		if err = db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("replay duplicated durable write: count=%d err=%v", count, err)
		}
	}
	if store.copied != 1 {
		t.Fatal("source copied more than once")
	}
}
