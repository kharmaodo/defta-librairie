CREATE TABLE book_inventory (
    book_id INTEGER PRIMARY KEY,
    library_id TEXT NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    low_stock_threshold INTEGER NOT NULL DEFAULT 5 CHECK (low_stock_threshold >= 0),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_at TEXT NOT NULL,
    FOREIGN KEY (book_id) REFERENCES defta(id) ON DELETE CASCADE,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT
);

CREATE TABLE inventory_movements (
    id TEXT PRIMARY KEY,
    book_id INTEGER NOT NULL,
    library_id TEXT NOT NULL,
    actor_user_id TEXT NOT NULL,
    movement_type TEXT NOT NULL CHECK (movement_type IN ('ENTRY', 'EXIT', 'ADJUSTMENT')),
    quantity_delta INTEGER NOT NULL CHECK (quantity_delta <> 0),
    quantity_before INTEGER NOT NULL CHECK (quantity_before >= 0),
    quantity_after INTEGER NOT NULL CHECK (quantity_after >= 0),
    reason TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (book_id) REFERENCES defta(id) ON DELETE RESTRICT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE RESTRICT,
    FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX idx_book_inventory_library_quantity
    ON book_inventory(library_id, quantity, book_id);

CREATE INDEX idx_inventory_movements_book_created
    ON inventory_movements(book_id, created_at DESC);

CREATE INDEX idx_inventory_movements_library_created
    ON inventory_movements(library_id, created_at DESC);

INSERT INTO book_inventory(book_id, library_id, quantity, low_stock_threshold, version, updated_at)
SELECT id, library_id, 0, 5, 1, COALESCE(updated_at, CURRENT_TIMESTAMP)
FROM defta
WHERE deleted_at IS NULL AND library_id IS NOT NULL;
