CREATE TABLE book_covers (
    id TEXT PRIMARY KEY,
    book_id INTEGER NOT NULL,
    library_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'READY', 'FAILED')),
    source_object_key TEXT NOT NULL UNIQUE,
    source_content_type TEXT NOT NULL CHECK (source_content_type IN ('image/jpeg', 'image/png')),
    source_format TEXT NOT NULL CHECK (source_format IN ('jpeg', 'png')),
    source_width INTEGER NOT NULL CHECK (source_width > 0),
    source_height INTEGER NOT NULL CHECK (source_height > 0),
    source_size INTEGER NOT NULL CHECK (source_size > 0),
    master_object_key TEXT,
    large_jpeg_object_key TEXT,
    large_webp_object_key TEXT,
    thumb_jpeg_object_key TEXT,
    thumb_webp_object_key TEXT,
    error_code TEXT,
    active INTEGER NOT NULL DEFAULT 0 CHECK (active IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (book_id) REFERENCES defta(id),
    FOREIGN KEY (library_id) REFERENCES libraries(id)
);

CREATE INDEX idx_book_covers_book_status
    ON book_covers(library_id, book_id, status, created_at DESC);

CREATE UNIQUE INDEX idx_book_covers_one_active
    ON book_covers(book_id)
    WHERE active = 1;

CREATE TABLE cover_processing_outbox (
    event_id TEXT PRIMARY KEY,
    cover_id TEXT NOT NULL,
    book_id INTEGER NOT NULL,
    library_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type = 'book.covers.process.v1'),
    schema_version INTEGER NOT NULL CHECK (schema_version = 1),
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TEXT NOT NULL,
    published_at TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (cover_id) REFERENCES book_covers(id),
    FOREIGN KEY (book_id) REFERENCES defta(id),
    FOREIGN KEY (library_id) REFERENCES libraries(id)
);

CREATE INDEX idx_cover_outbox_pending
    ON cover_processing_outbox(published_at, available_at, created_at);
