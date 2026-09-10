-- NULL means that the opening inventory cost is unknown.
ALTER TABLE book_inventory ADD COLUMN average_unit_cost REAL
    CHECK (average_unit_cost IS NULL OR (average_unit_cost >= 0 AND average_unit_cost < 1.0e308));
