-- Protect both transition orders; existing history is left untouched.
CREATE TRIGGER sales_cancel_completed_return_guard
BEFORE UPDATE OF status ON sales
WHEN NEW.status='CANCELLED' AND EXISTS (
    SELECT 1 FROM customer_returns WHERE sale_id=OLD.id AND status='COMPLETED'
)
BEGIN
    SELECT RAISE(ABORT, 'sale has completed customer returns');
END;

CREATE TRIGGER customer_returns_confirmed_sale_guard
BEFORE UPDATE OF status ON customer_returns
WHEN NEW.status='COMPLETED' AND NOT EXISTS (
    SELECT 1 FROM sales WHERE id=NEW.sale_id AND library_id=NEW.library_id AND status='CONFIRMED'
)
BEGIN
    SELECT RAISE(ABORT, 'customer return sale is unavailable');
END;
