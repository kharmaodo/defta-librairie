CREATE TABLE customers (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    reference TEXT NOT NULL,
    name TEXT NOT NULL CHECK (LENGTH(TRIM(name)) BETWEEN 2 AND 160),
    phone TEXT,
    email TEXT,
    address TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'DISABLED')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (id, library_id),
    UNIQUE (library_id, reference)
);

CREATE INDEX idx_customers_library_status_name
    ON customers(library_id, status, name COLLATE NOCASE);

CREATE INDEX idx_customers_library_phone
    ON customers(library_id, phone)
    WHERE phone IS NOT NULL AND phone <> '';

CREATE INDEX idx_customers_library_email
    ON customers(library_id, email COLLATE NOCASE)
    WHERE email IS NOT NULL AND email <> '';
