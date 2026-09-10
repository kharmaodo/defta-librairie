ALTER TABLE sales
    ADD COLUMN customer_id TEXT REFERENCES customers(id) ON DELETE RESTRICT;

CREATE INDEX idx_sales_customer_created
    ON sales(customer_id, created_at DESC)
    WHERE customer_id IS NOT NULL;

CREATE TRIGGER sales_customer_library_insert
BEFORE INSERT ON sales
WHEN NEW.customer_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1 FROM customers
    WHERE id = NEW.customer_id AND library_id = NEW.library_id
 )
BEGIN
    SELECT RAISE(ABORT, 'sale customer must belong to the sale library');
END;

CREATE TRIGGER sales_customer_library_update
BEFORE UPDATE OF customer_id, library_id ON sales
WHEN NEW.customer_id IS NOT NULL
 AND NOT EXISTS (
    SELECT 1 FROM customers
    WHERE id = NEW.customer_id AND library_id = NEW.library_id
 )
BEGIN
    SELECT RAISE(ABORT, 'sale customer must belong to the sale library');
END;
