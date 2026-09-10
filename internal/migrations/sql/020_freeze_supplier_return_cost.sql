-- Preserve unknown costs for historical shipments; never backfill from current CMP.
ALTER TABLE supplier_return_lines ADD COLUMN unit_cost_snapshot REAL
    CHECK (unit_cost_snapshot IS NULL OR (unit_cost_snapshot >= 0 AND unit_cost_snapshot < 1.0e308));
