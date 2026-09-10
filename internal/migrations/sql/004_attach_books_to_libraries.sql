ALTER TABLE defta ADD COLUMN library_id TEXT REFERENCES libraries(id) ON UPDATE CASCADE ON DELETE RESTRICT;
ALTER TABLE defta ADD COLUMN created_at TEXT;
ALTER TABLE defta ADD COLUMN updated_at TEXT;
ALTER TABLE defta ADD COLUMN deleted_at TEXT;
ALTER TABLE defta ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

UPDATE defta
SET library_id = '00000000-0000-0000-0000-000000000001',
    created_at = COALESCE(created_at, CURRENT_TIMESTAMP),
    updated_at = COALESCE(updated_at, CURRENT_TIMESTAMP)
WHERE library_id IS NULL;

CREATE INDEX idx_defta_library ON defta(library_id, id);
CREATE INDEX idx_defta_active_library ON defta(library_id, deleted_at);
