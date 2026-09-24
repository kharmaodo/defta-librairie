package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestBookManagementHTTPPersistsRelationalTags(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "book-tags-http.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if _, err = db.Exec(`
		INSERT INTO users(id, username, password_hash, role, status, created_at, updated_at)
		VALUES ('owner', 'owner', 'hash', 'OWNER_LIBRARY', 'ACTIVE', 'now', 'now');
		INSERT INTO libraries(id, name, owner_user_id, status, created_at, updated_at)
		VALUES ('library', 'Library', 'owner', 'ACTIVE', 'now', 'now');
		INSERT INTO library_tags(id, library_id, name, normalized_name, created_at, updated_at)
		VALUES ('tag-fiqh', 'library', 'Fiqh', 'fiqh', 'now', 'now'),
		       ('tag-arabic', 'library', 'Arabic', 'arabic', 'now', 'now');
	`); err != nil {
		t.Fatalf("seed tags: %v", err)
	}
	handler := NewBookManagementHandler(services.NewBookServiceWithRelations(
		repositories.NewBookRepository(db),
		repositories.NewBookTaxonomyRepository(db),
		repositories.NewBookTagRepository(db),
	))
	claims := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library"}
	claims.Subject = "owner"

	input := models.BookInput{Title: "Livre HTTP", Price: 2000, TagIDs: []string{"tag-fiqh", "tag-arabic"}}
	response := httptest.NewRecorder()
	handler.Create(response, relationalTagBookRequest(t, http.MethodPost, "/api/manage/books", input, claims))
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var created models.Book
	if err = json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode book: %v", err)
	}
	if len(created.TagIDs) != 2 {
		t.Fatalf("created tags=%v", created.TagIDs)
	}

	update := models.BookInput{Title: "Livre HTTP", Price: 2000, Version: created.Version, TagIDs: []string{"tag-arabic"}}
	request := relationalTagBookRequest(t, http.MethodPut, "/api/manage/books/1", update, claims)
	request.SetPathValue("id", "1")
	response = httptest.NewRecorder()
	handler.Update(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", response.Code, response.Body.String())
	}
	var updated models.Book
	if err = json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated book: %v", err)
	}
	if len(updated.TagIDs) != 1 || updated.TagIDs[0] != "tag-arabic" {
		t.Fatalf("updated tags=%v", updated.TagIDs)
	}
}

func relationalTagBookRequest(t *testing.T, method, target string, input models.BookInput, claims *auth.Claims) *http.Request {
	t.Helper()
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal book request: %v", err)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(auth.ContextWithClaims(request.Context(), claims))
}
