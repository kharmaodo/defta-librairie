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

func TestBookManagementHTTPPersistsTaxonomyRelations(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "books-taxonomy.db")+"?_foreign_keys=on")
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
	`); err != nil {
		t.Fatalf("seed owner and library: %v", err)
	}

	var fiqhID, nahwID, publisherID int
	if err = db.QueryRow(`SELECT id FROM categories WHERE code='fiqh'`).Scan(&fiqhID); err != nil {
		t.Fatalf("find fiqh: %v", err)
	}
	if err = db.QueryRow(`SELECT id FROM categories WHERE code='nahw'`).Scan(&nahwID); err != nil {
		t.Fatalf("find nahw: %v", err)
	}
	if err = db.QueryRow(`SELECT id FROM publishers WHERE code='dar-al-fikr'`).Scan(&publisherID); err != nil {
		t.Fatalf("find publisher: %v", err)
	}

	handler := NewBookManagementHandler(services.NewBookService(
		repositories.NewBookRepository(db),
		repositories.NewBookTaxonomyRepository(db),
	))
	claims := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library"}
	claims.Subject = "owner"

	createInput := models.BookInput{
		Title:             "Livre HTTP",
		Price:             2000,
		PublisherID:       &publisherID,
		CategoryIDs:       []int{fiqhID, nahwID},
		PrimaryCategoryID: &fiqhID,
	}
	createResponse := httptest.NewRecorder()
	handler.Create(createResponse, taxonomyBookRequest(t, http.MethodPost, "/api/manage/books", createInput, claims))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	var created models.Book
	if err = json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode created book: %v", err)
	}
	if !created.PublisherID.Valid || int(created.PublisherID.Int64) != publisherID ||
		!created.PrimaryCategoryID.Valid || int(created.PrimaryCategoryID.Int64) != fiqhID ||
		len(created.CategoryIDs) != 2 {
		t.Fatalf("created book taxonomy=%+v", created)
	}

	updateInput := models.BookInput{
		Title:             "Livre HTTP mis à jour",
		Price:             2500,
		Version:           created.Version,
		PublisherID:       &publisherID,
		CategoryIDs:       []int{nahwID},
		PrimaryCategoryID: &nahwID,
	}
	updateRequest := taxonomyBookRequest(t, http.MethodPut, "/api/manage/books/1", updateInput, claims)
	updateRequest.SetPathValue("id", "1")
	updateResponse := httptest.NewRecorder()
	handler.Update(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated models.Book
	if err = json.NewDecoder(updateResponse.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated book: %v", err)
	}
	if !updated.PrimaryCategoryID.Valid || int(updated.PrimaryCategoryID.Int64) != nahwID ||
		len(updated.CategoryIDs) != 1 || updated.CategoryIDs[0] != nahwID {
		t.Fatalf("updated book taxonomy=%+v", updated)
	}

	getRequest := taxonomyBookRequest(t, http.MethodGet, "/api/manage/books/1", models.BookInput{}, claims)
	getRequest.SetPathValue("id", "1")
	getResponse := httptest.NewRecorder()
	handler.Get(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getResponse.Code, getResponse.Body.String())
	}
	var fetched models.Book
	if err = json.NewDecoder(getResponse.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode fetched book: %v", err)
	}
	if !fetched.PublisherID.Valid || int(fetched.PublisherID.Int64) != publisherID ||
		len(fetched.CategoryIDs) != 1 || fetched.CategoryIDs[0] != nahwID {
		t.Fatalf("fetched book taxonomy=%+v", fetched)
	}

	listResponse := httptest.NewRecorder()
	handler.List(listResponse, taxonomyBookRequest(t, http.MethodGet, "/api/manage/books", models.BookInput{}, claims))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var listed struct {
		Results []models.Book `json:"results"`
	}
	if err = json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed books: %v", err)
	}
	if len(listed.Results) != 1 || len(listed.Results[0].CategoryIDs) != 1 ||
		listed.Results[0].CategoryIDs[0] != nahwID || listed.Results[0].HasActiveCover {
		t.Fatalf("listed book taxonomy=%+v", listed.Results)
	}

	if _, err = db.Exec(`UPDATE categories SET active=0 WHERE id=?`, fiqhID); err != nil {
		t.Fatalf("disable category: %v", err)
	}
	rejectedInput := models.BookInput{
		Title:             "Livre HTTP rejeté",
		Price:             updated.Price,
		Version:           updated.Version,
		CategoryIDs:       []int{fiqhID},
		PrimaryCategoryID: &fiqhID,
	}
	rejectedRequest := taxonomyBookRequest(t, http.MethodPut, "/api/manage/books/1", rejectedInput, claims)
	rejectedRequest.SetPathValue("id", "1")
	rejectedResponse := httptest.NewRecorder()
	handler.Update(rejectedResponse, rejectedRequest)
	if rejectedResponse.Code != http.StatusBadRequest {
		t.Fatalf("inactive category status=%d body=%s", rejectedResponse.Code, rejectedResponse.Body.String())
	}
}

func taxonomyBookRequest(t *testing.T, method, target string, input models.BookInput, claims *auth.Claims) *http.Request {
	t.Helper()
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal book request: %v", err)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(auth.ContextWithClaims(request.Context(), claims))
}
