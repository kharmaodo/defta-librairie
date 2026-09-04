ALTER TABLE suppliers
    ADD COLUMN normalized_name TEXT NOT NULL DEFAULT '';

UPDATE suppliers
SET normalized_name = LOWER(TRIM(name));

CREATE UNIQUE INDEX idx_suppliers_library_normalized_name
    ON suppliers(library_id, normalized_name);
