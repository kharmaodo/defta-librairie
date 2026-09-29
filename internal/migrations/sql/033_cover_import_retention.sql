-- A legal hold is scoped to one import job and always has an expiry.
CREATE TABLE cover_import_legal_holds (
    job_id TEXT PRIMARY KEY REFERENCES cover_import_jobs(id),
    reason TEXT NOT NULL CHECK (length(trim(reason)) > 0),
    actor_user_id TEXT NOT NULL REFERENCES users(id),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_cover_import_legal_holds_expiry ON cover_import_legal_holds(expires_at);
