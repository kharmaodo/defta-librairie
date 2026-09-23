CREATE TABLE book_submissions (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    actor_user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    auteur TEXT NOT NULL DEFAULT '',
    editeur TEXT NOT NULL DEFAULT '',
    price REAL NOT NULL CHECK (price >= 0),
    volume INTEGER NOT NULL CHECK (volume >= 0),
    status TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '',
    categorie TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    source_object_key TEXT NOT NULL UNIQUE,
    source_content_type TEXT NOT NULL CHECK (source_content_type IN ('image/jpeg', 'image/png')),
    source_format TEXT NOT NULL CHECK (source_format IN ('jpeg', 'png')),
    source_width INTEGER NOT NULL CHECK (source_width > 0),
    source_height INTEGER NOT NULL CHECK (source_height > 0),
    source_size INTEGER NOT NULL CHECK (source_size > 0),
    moderation_status TEXT NOT NULL CHECK (
        moderation_status IN (
            'PENDING_SCAN', 'SCANNING', 'APPROVED',
            'REJECTED', 'REVIEW_REQUIRED', 'FAILED'
        )
    ),
    moderation_score REAL CHECK (
        moderation_score IS NULL OR (moderation_score >= 0 AND moderation_score <= 1)
    ),
    moderation_model_version TEXT,
    decision_code TEXT,
    created_book_id INTEGER UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (library_id) REFERENCES libraries(id),
    FOREIGN KEY (created_book_id) REFERENCES defta(id)
);

CREATE INDEX idx_book_submissions_pending
    ON book_submissions(moderation_status, expires_at, created_at);

CREATE INDEX idx_book_submissions_library
    ON book_submissions(library_id, moderation_status, created_at DESC);

CREATE TABLE book_submission_outbox (
    event_id TEXT PRIMARY KEY,
    submission_id TEXT NOT NULL,
    library_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type = 'book.submissions.moderate.v1'),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TEXT NOT NULL,
    published_at TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (submission_id) REFERENCES book_submissions(id),
    FOREIGN KEY (library_id) REFERENCES libraries(id)
);

CREATE INDEX idx_book_submission_outbox_pending
    ON book_submission_outbox(published_at, available_at, created_at);
