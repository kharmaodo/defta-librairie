CREATE TABLE supplier_returns (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    purchase_id TEXT NOT NULL,
    supplier_id TEXT NOT NULL,
    reference TEXT NOT NULL,
    supplier_reference TEXT,
    reason TEXT NOT NULL CHECK (LENGTH(TRIM(reason)) BETWEEN 3 AND 1000),
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'SHIPPED', 'CANCELLED')),
    total_amount REAL NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    shipped_by TEXT,
    cancelled_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    shipped_at TEXT,
    cancelled_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (purchase_id) REFERENCES purchases(id) ON DELETE RESTRICT,
    FOREIGN KEY (supplier_id, library_id) REFERENCES suppliers(id, library_id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (shipped_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (cancelled_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (library_id, reference)
);

CREATE TABLE supplier_return_lines (
    id TEXT PRIMARY KEY,
    return_id TEXT NOT NULL,
    purchase_line_id TEXT NOT NULL,
    book_id INTEGER NOT NULL,
    title_snapshot TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_cost REAL NOT NULL CHECK (unit_cost >= 0),
    line_total REAL NOT NULL CHECK (line_total >= 0),
    created_at TEXT NOT NULL,
    FOREIGN KEY (return_id) REFERENCES supplier_returns(id) ON DELETE CASCADE,
    FOREIGN KEY (purchase_line_id) REFERENCES purchase_lines(id) ON DELETE RESTRICT,
    FOREIGN KEY (book_id) REFERENCES defta(id) ON DELETE RESTRICT,
    UNIQUE (return_id, purchase_line_id)
);

CREATE INDEX idx_supplier_returns_library_status_created
    ON supplier_returns(library_id, status, created_at DESC);

CREATE INDEX idx_supplier_returns_purchase_created
    ON supplier_returns(purchase_id, created_at DESC);

CREATE INDEX idx_supplier_returns_supplier_created
    ON supplier_returns(supplier_id, created_at DESC);

CREATE INDEX idx_supplier_return_lines_return
    ON supplier_return_lines(return_id, id);

CREATE INDEX idx_supplier_return_lines_purchase_line
    ON supplier_return_lines(purchase_line_id, return_id);

CREATE TRIGGER supplier_return_purchase_insert
BEFORE INSERT ON supplier_returns
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1
        FROM purchases p
        WHERE p.id = NEW.purchase_id
          AND p.library_id = NEW.library_id
          AND p.supplier_id = NEW.supplier_id
          AND p.status = 'RECEIVED'
    ) THEN RAISE(ABORT, 'supplier return purchase unavailable') END;
END;

CREATE TRIGGER supplier_return_purchase_update
BEFORE UPDATE OF purchase_id, library_id, supplier_id ON supplier_returns
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1
        FROM purchases p
        WHERE p.id = NEW.purchase_id
          AND p.library_id = NEW.library_id
          AND p.supplier_id = NEW.supplier_id
          AND p.status = 'RECEIVED'
    ) THEN RAISE(ABORT, 'supplier return purchase unavailable') END;
END;

CREATE TRIGGER supplier_return_line_insert
BEFORE INSERT ON supplier_return_lines
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1
        FROM supplier_returns r
        JOIN purchase_lines pl ON pl.purchase_id = r.purchase_id
        WHERE r.id = NEW.return_id
          AND r.status = 'DRAFT'
          AND pl.id = NEW.purchase_line_id
          AND pl.book_id = NEW.book_id
    ) THEN RAISE(ABORT, 'supplier return line unavailable') END;

    SELECT CASE WHEN NEW.quantity + COALESCE((
        SELECT SUM(existing.quantity)
        FROM supplier_return_lines existing
        JOIN supplier_returns other_return ON other_return.id = existing.return_id
        WHERE existing.purchase_line_id = NEW.purchase_line_id
          AND other_return.status IN ('DRAFT', 'SHIPPED')
          AND other_return.id <> NEW.return_id
    ), 0) > (
        SELECT quantity FROM purchase_lines WHERE id = NEW.purchase_line_id
    ) THEN RAISE(ABORT, 'supplier return quantity exceeds received quantity') END;
END;

CREATE TRIGGER supplier_return_not_empty_before_shipping
BEFORE UPDATE OF status ON supplier_returns
WHEN NEW.status = 'SHIPPED' AND OLD.status = 'DRAFT'
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM supplier_return_lines WHERE return_id = NEW.id
    ) THEN RAISE(ABORT, 'supplier return cannot be empty') END;
END;
