package handlers

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestCustomerHistoryHTTPValidation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO users(id,username,password_hash,role,created_at,updated_at) VALUES('h','h','unused','OWNER_LIBRARY','now','now');
        INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at) VALUES('h','History','h','now','now');
        INSERT INTO customers(id,library_id,reference,name,created_by,created_at,updated_at) VALUES('h','h','H','History','h','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewCustomerHandler(services.NewCustomerService(repositories.NewCustomerRepository(db)))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/manage/customers/{id}/sales", handler.SaleHistory)
	for _, query := range []string{"", "?offset=-1", "?limit=101", "?offset=abc", "?limit=1&limit=2", "?from=bad", "?libraryId=other"} {
		req := httptest.NewRequest("GET", "/api/manage/customers/h/sales"+query, nil)
		req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "h"}))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		expected := http.StatusBadRequest
		if query == "" {
			expected = http.StatusOK
		}
		if w.Code != expected || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("query=%s status=%d body=%s", query, w.Code, w.Body.String())
		}
		var v map[string]interface{}
		if err = json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if query == "" {
			rows, ok := v["results"].([]interface{})
			if !ok || len(rows) != 0 {
				t.Fatalf("empty response=%v", v)
			}
		} else if v["error"] != "invalid_customer_history" {
			t.Fatalf("error=%v", v)
		}
	}
}
