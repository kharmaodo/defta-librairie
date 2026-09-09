-- Historical sales and unknown inventory costs remain NULL.
ALTER TABLE sale_lines ADD COLUMN unit_cost_snapshot REAL
    CHECK (unit_cost_snapshot IS NULL OR (unit_cost_snapshot >= 0 AND unit_cost_snapshot < 1.0e308));
