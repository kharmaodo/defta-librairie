CREATE TABLE customer_returns (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    customer_id TEXT,
    reference TEXT NOT NULL,
    reason TEXT NOT NULL CHECK (LENGTH(TRIM(reason)) BETWEEN 3 AND 1000),
    status TEXT NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT', 'COMPLETED', 'CANCELLED')),
    resolution TEXT NOT NULL
        CHECK (resolution IN ('REFUND', 'CREDIT_NOTE')),
    total_amount REAL NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    completed_by TEXT,
    cancelled_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT,
    cancelled_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE RESTRICT,
    FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (completed_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (cancelled_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (id, library_id),
    UNIQUE (library_id, reference)
);

CREATE TABLE customer_return_lines (
    id TEXT PRIMARY KEY,
    return_id TEXT NOT NULL,
    sale_line_id TEXT NOT NULL,
    book_id INTEGER NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price REAL NOT NULL CHECK (unit_price >= 0),
    line_total REAL NOT NULL CHECK (line_total >= 0),
    created_at TEXT NOT NULL,
    FOREIGN KEY (return_id) REFERENCES customer_returns(id) ON DELETE CASCADE,
    FOREIGN KEY (sale_line_id) REFERENCES sale_lines(id) ON DELETE RESTRICT,
    FOREIGN KEY (book_id) REFERENCES defta(id) ON DELETE RESTRICT,
    UNIQUE (return_id, sale_line_id)
);

CREATE TABLE return_settlements (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    return_id TEXT NOT NULL,
    method TEXT NOT NULL
        CHECK (method IN ('CASH', 'MOBILE_MONEY', 'CARD', 'CREDIT_NOTE')),
    amount REAL NOT NULL CHECK (amount > 0),
    external_reference TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'ISSUED'
        CHECK (status IN ('ISSUED', 'VOIDED')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    issued_by TEXT NOT NULL,
    voided_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    voided_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (return_id, library_id)
        REFERENCES customer_returns(id, library_id) ON DELETE RESTRICT,
    FOREIGN KEY (issued_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (voided_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX idx_customer_returns_library_status_created
    ON customer_returns(library_id, status, created_at DESC);

CREATE INDEX idx_customer_returns_sale_created
    ON customer_returns(sale_id, created_at DESC);

CREATE INDEX idx_customer_return_lines_return
    ON customer_return_lines(return_id, id);

CREATE INDEX idx_customer_return_lines_sale_line
    ON customer_return_lines(sale_line_id, return_id);

CREATE INDEX idx_return_settlements_return_created
    ON return_settlements(return_id, created_at DESC);

CREATE UNIQUE INDEX idx_return_settlements_external_reference
    ON return_settlements(library_id, method, external_reference)
    WHERE external_reference IS NOT NULL
      AND external_reference <> ''
      AND status = 'ISSUED';

CREATE TRIGGER customer_returns_context_insert
BEFORE INSERT ON customer_returns
WHEN NOT EXISTS (
    SELECT 1 FROM sales s
    WHERE s.id = NEW.sale_id
      AND s.library_id = NEW.library_id
      AND s.status = 'CONFIRMED'
      AND (NEW.customer_id IS NULL OR s.customer_id = NEW.customer_id)
)
BEGIN
    SELECT RAISE(ABORT, 'customer return sale is unavailable');
END;

CREATE TRIGGER customer_return_lines_context_insert
BEFORE INSERT ON customer_return_lines
WHEN NOT EXISTS (
    SELECT 1
    FROM customer_returns r
    JOIN sale_lines sl
      ON sl.id = NEW.sale_line_id
     AND sl.sale_id = r.sale_id
     AND sl.book_id = NEW.book_id
    WHERE r.id = NEW.return_id
      AND r.status = 'DRAFT'
      AND NEW.quantity <= sl.quantity
      AND NEW.unit_price = sl.unit_price
      AND NEW.line_total = NEW.quantity * sl.unit_price
)
BEGIN
    SELECT RAISE(ABORT, 'customer return line is invalid');
END;

CREATE TRIGGER customer_return_lines_context_update
BEFORE UPDATE ON customer_return_lines
WHEN NOT EXISTS (
    SELECT 1
    FROM customer_returns r
    JOIN sale_lines sl
      ON sl.id = NEW.sale_line_id
     AND sl.sale_id = r.sale_id
     AND sl.book_id = NEW.book_id
    WHERE r.id = NEW.return_id
      AND r.status = 'DRAFT'
      AND NEW.quantity <= sl.quantity
      AND NEW.unit_price = sl.unit_price
      AND NEW.line_total = NEW.quantity * sl.unit_price
)
BEGIN
    SELECT RAISE(ABORT, 'customer return line is invalid');
END;

CREATE TRIGGER customer_return_lines_context_delete
BEFORE DELETE ON customer_return_lines
WHEN NOT EXISTS (
    SELECT 1 FROM customer_returns
    WHERE id = OLD.return_id AND status = 'DRAFT'
)
BEGIN
    SELECT RAISE(ABORT, 'completed customer return cannot be modified');
END;

CREATE TRIGGER customer_return_lines_total_insert
AFTER INSERT ON customer_return_lines
BEGIN
    UPDATE customer_returns
    SET total_amount = (
        SELECT COALESCE(SUM(line_total), 0)
        FROM customer_return_lines
        WHERE return_id = NEW.return_id
    )
    WHERE id = NEW.return_id;
END;

CREATE TRIGGER customer_return_lines_total_update
AFTER UPDATE ON customer_return_lines
BEGIN
    UPDATE customer_returns
    SET total_amount = (
        SELECT COALESCE(SUM(line_total), 0)
        FROM customer_return_lines
        WHERE return_id = NEW.return_id
    )
    WHERE id = NEW.return_id;
END;

CREATE TRIGGER customer_return_lines_total_delete
AFTER DELETE ON customer_return_lines
BEGIN
    UPDATE customer_returns
    SET total_amount = (
        SELECT COALESCE(SUM(line_total), 0)
        FROM customer_return_lines
        WHERE return_id = OLD.return_id
    )
    WHERE id = OLD.return_id;
END;

CREATE TRIGGER customer_returns_complete_guard
BEFORE UPDATE OF status ON customer_returns
WHEN NEW.status = 'COMPLETED'
 AND (
    NOT EXISTS (SELECT 1 FROM customer_return_lines WHERE return_id = NEW.id)
    OR EXISTS (
        SELECT 1
        FROM customer_return_lines current_line
        JOIN sale_lines sold_line ON sold_line.id = current_line.sale_line_id
        WHERE current_line.return_id = NEW.id
          AND current_line.quantity + COALESCE((
              SELECT SUM(previous_line.quantity)
              FROM customer_return_lines previous_line
              JOIN customer_returns previous_return ON previous_return.id = previous_line.return_id
              WHERE previous_line.sale_line_id = current_line.sale_line_id
                AND previous_return.status = 'COMPLETED'
                AND previous_return.id <> NEW.id
          ), 0) > sold_line.quantity
    )
 )
BEGIN
    SELECT RAISE(ABORT, 'customer return quantity exceeds sold quantity');
END;

CREATE TRIGGER return_settlements_context_insert
BEFORE INSERT ON return_settlements
WHEN NOT EXISTS (
    SELECT 1 FROM customer_returns r
    WHERE r.id = NEW.return_id
      AND r.library_id = NEW.library_id
      AND r.status = 'COMPLETED'
      AND ((r.resolution = 'CREDIT_NOTE' AND NEW.method = 'CREDIT_NOTE')
        OR (r.resolution = 'REFUND' AND NEW.method <> 'CREDIT_NOTE'))
)
BEGIN
    SELECT RAISE(ABORT, 'customer return settlement is unavailable');
END;

CREATE TRIGGER return_settlements_amount_insert
BEFORE INSERT ON return_settlements
WHEN NEW.status = 'ISSUED'
 AND COALESCE((
     SELECT SUM(amount) FROM return_settlements
     WHERE return_id = NEW.return_id AND status = 'ISSUED'
 ), 0) + NEW.amount > (
     SELECT total_amount FROM customer_returns WHERE id = NEW.return_id
 )
BEGIN
    SELECT RAISE(ABORT, 'customer return settlement exceeds return amount');
END;

CREATE VIEW customer_return_balances AS
SELECT
    r.id AS return_id,
    r.library_id,
    r.total_amount,
    COALESCE(SUM(CASE WHEN s.status = 'ISSUED' THEN s.amount ELSE 0 END), 0) AS settled_amount,
    r.total_amount - COALESCE(SUM(CASE WHEN s.status = 'ISSUED' THEN s.amount ELSE 0 END), 0) AS remaining_amount,
    CASE
        WHEN COALESCE(SUM(CASE WHEN s.status = 'ISSUED' THEN s.amount ELSE 0 END), 0) = 0 THEN 'PENDING'
        WHEN COALESCE(SUM(CASE WHEN s.status = 'ISSUED' THEN s.amount ELSE 0 END), 0) < r.total_amount THEN 'PARTIALLY_SETTLED'
        ELSE 'SETTLED'
    END AS settlement_status
FROM customer_returns r
LEFT JOIN return_settlements s ON s.return_id = r.id
GROUP BY r.id, r.library_id, r.total_amount;
