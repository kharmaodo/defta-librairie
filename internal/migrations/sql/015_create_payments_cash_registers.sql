CREATE TABLE cash_registers (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (LENGTH(TRIM(name)) BETWEEN 2 AND 80),
    normalized_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'DISABLED')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT,
    UNIQUE (id, library_id),
    UNIQUE (library_id, normalized_name)
);

CREATE TABLE payments (
    id TEXT PRIMARY KEY,
    library_id TEXT NOT NULL,
    sale_id TEXT NOT NULL,
    cash_register_id TEXT NOT NULL,
    method TEXT NOT NULL
        CHECK (method IN ('CASH', 'MOBILE_MONEY', 'CARD')),
    amount REAL NOT NULL CHECK (amount > 0),
    external_reference TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'RECORDED'
        CHECK (status IN ('RECORDED', 'VOIDED')),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    recorded_by TEXT NOT NULL,
    voided_by TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    voided_at TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (sale_id) REFERENCES sales(id) ON DELETE RESTRICT,
    FOREIGN KEY (cash_register_id, library_id)
        REFERENCES cash_registers(id, library_id) ON DELETE RESTRICT,
    FOREIGN KEY (recorded_by) REFERENCES users(id) ON DELETE RESTRICT,
    FOREIGN KEY (voided_by) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX idx_cash_registers_library_status_name
    ON cash_registers(library_id, status, name COLLATE NOCASE);

CREATE INDEX idx_payments_sale_created
    ON payments(sale_id, created_at DESC);

CREATE INDEX idx_payments_register_created
    ON payments(cash_register_id, created_at DESC);

CREATE INDEX idx_payments_library_method_created
    ON payments(library_id, method, created_at DESC);

CREATE UNIQUE INDEX idx_payments_external_reference
    ON payments(library_id, method, external_reference)
    WHERE external_reference IS NOT NULL
      AND external_reference <> ''
      AND status = 'RECORDED';

CREATE TRIGGER payments_context_insert
BEFORE INSERT ON payments
WHEN NOT EXISTS (
    SELECT 1
    FROM sales s
    JOIN cash_registers c
      ON c.id = NEW.cash_register_id
     AND c.library_id = NEW.library_id
     AND c.status = 'ACTIVE'
    WHERE s.id = NEW.sale_id
      AND s.library_id = NEW.library_id
      AND s.status = 'CONFIRMED'
)
BEGIN
    SELECT RAISE(ABORT, 'payment sale or cash register is unavailable');
END;

CREATE TRIGGER payments_overpayment_insert
BEFORE INSERT ON payments
WHEN NEW.status = 'RECORDED'
 AND (
    COALESCE((
        SELECT SUM(amount)
        FROM payments
        WHERE sale_id = NEW.sale_id AND status = 'RECORDED'
    ), 0) + NEW.amount
 ) > (
    SELECT total_amount FROM sales WHERE id = NEW.sale_id
 )
BEGIN
    SELECT RAISE(ABORT, 'payment exceeds sale remaining amount');
END;

CREATE VIEW sale_payment_balances AS
SELECT
    s.id AS sale_id,
    s.library_id,
    s.total_amount,
    COALESCE(SUM(CASE WHEN p.status = 'RECORDED' THEN p.amount ELSE 0 END), 0) AS paid_amount,
    s.total_amount - COALESCE(SUM(CASE WHEN p.status = 'RECORDED' THEN p.amount ELSE 0 END), 0) AS remaining_amount,
    CASE
        WHEN COALESCE(SUM(CASE WHEN p.status = 'RECORDED' THEN p.amount ELSE 0 END), 0) = 0 THEN 'UNPAID'
        WHEN COALESCE(SUM(CASE WHEN p.status = 'RECORDED' THEN p.amount ELSE 0 END), 0) < s.total_amount THEN 'PARTIALLY_PAID'
        ELSE 'PAID'
    END AS payment_status
FROM sales s
LEFT JOIN payments p ON p.sale_id = s.id
GROUP BY s.id, s.library_id, s.total_amount;
