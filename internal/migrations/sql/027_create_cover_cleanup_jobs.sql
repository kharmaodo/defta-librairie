CREATE TABLE cover_object_cleanup_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cover_id TEXT NOT NULL,
    library_id TEXT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    object_kind TEXT NOT NULL CHECK (object_kind IN ('SOURCE', 'GENERATED')),
    available_at TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    locked_by TEXT,
    locked_until TEXT,
    completed_at TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_cover_cleanup_available
    ON cover_object_cleanup_jobs(completed_at, available_at, locked_until, id);
