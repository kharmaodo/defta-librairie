CREATE TABLE sales (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    reference TEXT NOT NULL,
    customer_name TEXT,
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'CONFIRMED', 'CANCELLED')),
    total_amount REAL NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    confirmed_by TEXT,
    cancelled_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    confirmed_at TEXT,
    cancelled_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (confirmed_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (cancelled_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (library_id, reference)
);

CREATE TABLE sale_lines (
    id TEXT PRIMARY KEY,
    sale_id TEXT NOT NULL,
    book_id INTEGER NOT NULL,
    title_snapshot TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price REAL NOT NULL CHECK (unit_price >= 0),
    line_total REAL NOT NULL CHECK (line_total >= 0),
    created_at TEXT NOT NULL,
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE CASCADE,
    FOREIGN KEY (book_id) REFERENCES defta(id) ON DELETE RESTRICT,
    UNIQUE (sale_id, book_id)
);

CREATE INDEX idx_sales_library_status_created
    ON sales(library_id, status, created_at DESC);

CREATE INDEX idx_sales_created
    ON sales(created_at DESC);

CREATE INDEX idx_sale_lines_sale
    ON sale_lines(sale_id, id);

CREATE INDEX idx_sale_lines_book
    ON sale_lines(book_id, sale_id);
