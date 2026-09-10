CREATE TABLE suppliers (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (LENGTH(TRIM(name)) BETWEEN 2 AND 160),
    contact_name TEXT,
    phone TEXT,
    email TEXT,
    address TEXT,
    status TEXT NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'DISABLED')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (id, library_id),
    UNIQUE (library_id, name COLLATE NOCASE)
);

CREATE TABLE purchases (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    reference TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'RECEIVED', 'CANCELLED')),
    total_amount REAL NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    received_by TEXT,
    cancelled_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    received_at TEXT,
    cancelled_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (supplier_id, library_id) REFERENCES suppliers(id, library_id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (received_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (cancelled_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (library_id, reference)
);

CREATE TABLE purchase_lines (
    id TEXT PRIMARY KEY,
    purchase_id TEXT NOT NULL,
    book_id INTEGER NOT NULL,
    title_snapshot TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_cost REAL NOT NULL CHECK (unit_cost >= 0),
    line_total REAL NOT NULL CHECK (line_total >= 0),
    created_at TEXT NOT NULL,
    FOREIGN KEY (purchase_id) REFERENCES purchases(id) ON DELETE CASCADE,
    FOREIGN KEY (book_id) REFERENCES defta(id) ON DELETE RESTRICT,
    UNIQUE (purchase_id, book_id)
);

CREATE INDEX idx_suppliers_library_status_name
    ON suppliers(library_id, status, name COLLATE NOCASE);

CREATE INDEX idx_purchases_library_status_created
    ON purchases(library_id, status, created_at DESC);

CREATE INDEX idx_purchases_supplier_created
    ON purchases(supplier_id, created_at DESC);

CREATE INDEX idx_purchase_lines_purchase
    ON purchase_lines(purchase_id, id);

CREATE INDEX idx_purchase_lines_book
    ON purchase_lines(book_id, purchase_id);
