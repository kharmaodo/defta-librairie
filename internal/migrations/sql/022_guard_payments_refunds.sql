-- Keep financial records explicit: no automatic voids or historical rewrites.
CREATE TRIGGER sales_cancel_recorded_payment_guard
BEFORE UPDATE OF status ON sales
WHEN NEW.status='CANCELLED' AND EXISTS (
    SELECT 1 FROM payments WHERE sale_id=OLD.id AND status='RECORDED'
)
BEGIN
    SELECT RAISE(ABORT, 'sale has recorded payments');
END;

CREATE TRIGGER monetary_refund_paid_amount_guard
BEFORE INSERT ON return_settlements
WHEN NEW.status='ISSUED' AND NEW.method IN ('CASH','MOBILE_MONEY','CARD')
 AND NEW.amount + COALESCE((
    SELECT SUM(rs.amount) FROM return_settlements rs
    JOIN customer_returns r ON r.id=rs.return_id
    WHERE r.sale_id=(SELECT sale_id FROM customer_returns WHERE id=NEW.return_id)
      AND rs.status='ISSUED' AND rs.method IN ('CASH','MOBILE_MONEY','CARD')
 ),0) > COALESCE((
    SELECT SUM(p.amount) FROM payments p
    WHERE p.sale_id=(SELECT sale_id FROM customer_returns WHERE id=NEW.return_id)
      AND p.status='RECORDED'
 ),0)
BEGIN
    SELECT RAISE(ABORT, 'monetary refund exceeds recorded payments');
END;

CREATE TRIGGER payment_void_refund_guard
BEFORE UPDATE OF status ON payments
WHEN OLD.status='RECORDED' AND NEW.status='VOIDED'
 AND COALESCE((
    SELECT SUM(rs.amount) FROM return_settlements rs
    JOIN customer_returns r ON r.id=rs.return_id
    WHERE r.sale_id=OLD.sale_id AND rs.status='ISSUED'
      AND rs.method IN ('CASH','MOBILE_MONEY','CARD')
 ),0) > COALESCE((
    SELECT SUM(p.amount) FROM payments p
    WHERE p.sale_id=OLD.sale_id AND p.status='RECORDED' AND p.id<>OLD.id
 ),0)
BEGIN
    SELECT RAISE(ABORT, 'payment void would uncover issued refunds');
END;
