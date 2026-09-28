-- v1.7.0: importation groupée de couvertures existantes.
-- Ce domaine est volontairement distinct de book_submissions, qui crée un livre.

CREATE TABLE cover_imports (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    actor_user_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED', 'CANCELLED')),
    total_files INTEGER NOT NULL CHECK (total_files BETWEEN 1 AND 100),
    accepted_files INTEGER NOT NULL DEFAULT 0 CHECK (accepted_files BETWEEN 0 AND total_files),
    rejected_files INTEGER NOT NULL DEFAULT 0 CHECK (rejected_files BETWEEN 0 AND total_files),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id),
    FOREIGN KEY (actor_user_id) REFERENCES users(id)
);

CREATE INDEX idx_cover_imports_library_created
    ON cover_imports(library_id, created_at DESC);

CREATE TABLE cover_import_jobs (
    id TEXT PRIMARY KEY,
    import_id TEXT NOT NULL,
    library_id TEXT NOT NULL,
    actor_user_id TEXT NOT NULL,
    source_object_key TEXT NOT NULL UNIQUE,
    source_content_type TEXT NOT NULL CHECK (source_content_type IN ('image/jpeg', 'image/png')),
    source_format TEXT NOT NULL CHECK (source_format IN ('jpeg', 'png')),
    source_width INTEGER NOT NULL CHECK (source_width > 0),
    source_height INTEGER NOT NULL CHECK (source_height > 0),
    source_size INTEGER NOT NULL CHECK (source_size > 0 AND source_size <= 10485760),
    input_sha256 TEXT NOT NULL CHECK (length(input_sha256) = 64),
    status TEXT NOT NULL CHECK (status IN (
        'PENDING_SCAN', 'NSFW_SCANNING', 'NSFW_DECIDED', 'OCR_PENDING',
        'OCR_PROCESSING', 'MATCHING', 'REVIEW_REQUIRED', 'READY',
        'REJECTED', 'QUARANTINED', 'FAILED', 'CANCELLED'
    )),
    nsfw_decision TEXT CHECK (nsfw_decision IN ('SAFE', 'REVIEW', 'UNSAFE')),
    nsfw_policy_version TEXT,
    target_book_id INTEGER,
    decision_code TEXT,
    failure_code TEXT,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    terminal_at TEXT,
    FOREIGN KEY (import_id) REFERENCES cover_imports(id),
    FOREIGN KEY (library_id) REFERENCES libraries(id),
    FOREIGN KEY (actor_user_id) REFERENCES users(id),
    FOREIGN KEY (target_book_id) REFERENCES defta(id)
);

CREATE INDEX idx_cover_import_jobs_import
    ON cover_import_jobs(import_id, created_at);
CREATE INDEX idx_cover_import_jobs_library_status
    ON cover_import_jobs(library_id, status, created_at DESC);
CREATE INDEX idx_cover_import_jobs_expiry
    ON cover_import_jobs(status, expires_at);

CREATE TABLE cover_import_outbox (
    event_id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    library_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'cover.imports.moderate.v1', 'cover.imports.ocr.v1', 'cover.imports.match.v1'
    )),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TEXT NOT NULL,
    published_at TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (job_id) REFERENCES cover_import_jobs(id),
    FOREIGN KEY (library_id) REFERENCES libraries(id)
);

CREATE INDEX idx_cover_import_outbox_pending
    ON cover_import_outbox(published_at, available_at, created_at);

CREATE TABLE cover_import_ocr_results (
    job_id TEXT PRIMARY KEY,
    engine TEXT NOT NULL,
    engine_version TEXT NOT NULL,
    language TEXT NOT NULL CHECK (language = 'ara'),
    text_raw TEXT NOT NULL,
    text_normalized TEXT NOT NULL,
    confidence REAL CHECK (confidence IS NULL OR (confidence >= 0 AND confidence <= 1)),
    title TEXT,
    auteur TEXT,
    editeur TEXT,
    isbn13 TEXT CHECK (isbn13 IS NULL OR length(isbn13) = 13),
    completed_at TEXT NOT NULL,
    FOREIGN KEY (job_id) REFERENCES cover_import_jobs(id)
);

CREATE TABLE cover_import_candidate_matches (
    job_id TEXT NOT NULL,
    book_id INTEGER NOT NULL,
    rank INTEGER NOT NULL CHECK (rank BETWEEN 1 AND 5),
    fts_score REAL,
    vector_score REAL,
    explanation_json TEXT NOT NULL CHECK (json_valid(explanation_json)),
    created_at TEXT NOT NULL,
    PRIMARY KEY (job_id, book_id),
    UNIQUE (job_id, rank),
    FOREIGN KEY (job_id) REFERENCES cover_import_jobs(id),
    FOREIGN KEY (book_id) REFERENCES defta(id)
);

CREATE TABLE cover_import_review_decisions (
    job_id TEXT PRIMARY KEY,
    actor_user_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('ACCEPT', 'REJECT')),
    book_id INTEGER,
    reason TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (job_id) REFERENCES cover_import_jobs(id),
    FOREIGN KEY (actor_user_id) REFERENCES users(id),
    FOREIGN KEY (book_id) REFERENCES defta(id),
    CHECK ((action = 'ACCEPT' AND book_id IS NOT NULL) OR (action = 'REJECT' AND book_id IS NULL))
);
