package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestHealthChecks(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	h := NewHealthHandler(db)
	check := func(handler http.HandlerFunc, ctx context.Context, code int, status, reason string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		handler(w, req)
		var v map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if w.Code != code || v["status"] != status || v["reason"] != reason || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	}
	ctx := context.Background()
	check(h.Live, ctx, 200, "alive", "")
	check(h.Ready, ctx, 503, "not_ready", "database_unavailable")
	_, err = db.Exec(`CREATE TABLE libraries(id TEXT PRIMARY KEY); CREATE TABLE schema_migrations(version TEXT PRIMARY KEY); INSERT INTO schema_migrations VALUES('test');`)
	if err != nil {
		t.Fatal(err)
	}
	check(h.Ready, ctx, 200, "ready", "")
	// A saturated pool must honor cancellation without creating a leaked goroutine.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	check(h.Ready, deadline, 503, "not_ready", "database_unavailable")
	cancel()
	conn.Close()
	check(h.Ready, ctx, 200, "ready", "")
	h.BeginShutdown()
	check(h.Ready, ctx, 503, "not_ready", "shutting_down")
	check(h.Live, ctx, 200, "alive", "")
	db.Close()
	fresh := NewHealthHandler(db)
	check(fresh.Ready, ctx, 503, "not_ready", "database_unavailable")
	check(fresh.Live, ctx, 200, "alive", "")
	check(NewHealthHandler(nil).Ready, ctx, 503, "not_ready", "database_unavailable")
}
