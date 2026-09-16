ALTER TABLE cover_processing_outbox
    ADD COLUMN locked_by TEXT;

ALTER TABLE cover_processing_outbox
    ADD COLUMN locked_until TEXT;

CREATE INDEX idx_cover_outbox_claimable
    ON cover_processing_outbox(published_at, available_at, locked_until, created_at);
