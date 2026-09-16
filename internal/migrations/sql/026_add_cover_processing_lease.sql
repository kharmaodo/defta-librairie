ALTER TABLE book_covers
    ADD COLUMN processing_by TEXT;

ALTER TABLE book_covers
    ADD COLUMN processing_until TEXT;

ALTER TABLE book_covers
    ADD COLUMN processing_attempts INTEGER NOT NULL DEFAULT 0
    CHECK (processing_attempts >= 0);

CREATE INDEX idx_book_covers_processing_lease
    ON book_covers(status, processing_until, updated_at);
