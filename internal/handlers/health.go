package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthHandler separates process liveness from database readiness.
type HealthHandler struct {
	db       *sql.DB
	draining atomic.Bool
}

func NewHealthHandler(db *sql.DB) *HealthHandler { return &HealthHandler{db: db} }
func (h *HealthHandler) BeginShutdown()          { h.draining.Store(true) }
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeAuthJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.draining.Load() {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "shutting_down"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	// Read actual application tables; Ping alone could succeed on an empty DB.
	var libraries, migrations int
	if h.db == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database_unavailable"})
		return
	}
	err := h.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM libraries WHERE id='00000000-0000-0000-0000-000000000001'), (SELECT COUNT(*) FROM schema_migrations)`).Scan(&libraries, &migrations)
	if h.draining.Load() {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "shutting_down"})
		return
	}
	if err != nil || migrations == 0 {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database_unavailable"})
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
