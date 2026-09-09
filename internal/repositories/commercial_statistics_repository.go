package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CommercialStatistics describes activity in a half-open interval [from, to).
// NetMargin is nil when any event has an unknown historical cost.
type CommercialStatistics struct {
	GrossSales        float64  `json:"grossSales"`
	Cancellations     float64  `json:"cancellations"`
	CustomerReturns   float64  `json:"customerReturns"`
	NetSales          float64  `json:"netSales"`
	KnownCost         float64  `json:"knownCost"`
	UnknownCostEvents int      `json:"unknownCostEvents"`
	NetMargin         *float64 `json:"netMargin"`
}

type CommercialStatisticsRepository struct{ db *sql.DB }

func NewCommercialStatisticsRepository(db *sql.DB) *CommercialStatisticsRepository {
	return &CommercialStatisticsRepository{db: db}
}

// Summary requires an explicit library scope. The caller must authorize it.
func (r *CommercialStatisticsRepository) Summary(ctx context.Context, libraryID string, from, to time.Time) (CommercialStatistics, error) {
	var result CommercialStatistics
	if libraryID == "" || from.IsZero() || to.IsZero() || !from.Before(to) {
		return result, errors.New("library and valid statistics interval required")
	}
	err := r.db.QueryRowContext(ctx, commercialStatisticsSQL, libraryID,
		from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)).Scan(
		&result.GrossSales, &result.Cancellations, &result.CustomerReturns,
		&result.KnownCost, &result.UnknownCostEvents)
	if err != nil {
		return result, err
	}
	result.NetSales = result.GrossSales - result.Cancellations - result.CustomerReturns
	if result.UnknownCostEvents == 0 {
		margin := result.NetSales - result.KnownCost
		result.NetMargin = &margin
	}
	return result, nil
}

// julianday compares timestamps chronologically, including offsets and fractions.
const commercialStatisticsSQL = `
WITH scope AS (SELECT ? AS library_id, julianday(?) AS start_at, julianday(?) AS end_at),
events AS (
 SELECT 'SALE' AS kind, s.confirmed_at AS event_at, sl.line_total AS amount,
        sl.quantity * sl.unit_cost_snapshot AS cost
 FROM sales s JOIN sale_lines sl ON sl.sale_id=s.id JOIN scope p ON p.library_id=s.library_id
 WHERE s.status IN ('CONFIRMED','CANCELLED') AND s.confirmed_at IS NOT NULL
 UNION ALL
 SELECT 'CANCEL', s.cancelled_at, -sl.line_total, -sl.quantity * sl.unit_cost_snapshot
 FROM sales s JOIN sale_lines sl ON sl.sale_id=s.id JOIN scope p ON p.library_id=s.library_id
 WHERE s.status='CANCELLED' AND s.confirmed_at IS NOT NULL
 UNION ALL
 SELECT 'RETURN', r.completed_at, -rl.line_total, -rl.quantity * sl.unit_cost_snapshot
 FROM customer_returns r JOIN customer_return_lines rl ON rl.return_id=r.id
 JOIN sale_lines sl ON sl.id=rl.sale_line_id AND sl.sale_id=r.sale_id AND sl.book_id=rl.book_id
 JOIN sales s ON s.id=r.sale_id AND s.library_id=r.library_id
 JOIN scope p ON p.library_id=r.library_id
 WHERE r.status='COMPLETED'
)
SELECT COALESCE(SUM(CASE WHEN kind='SALE' THEN amount ELSE 0 END),0),
       COALESCE(SUM(CASE WHEN kind='CANCEL' THEN -amount ELSE 0 END),0),
       COALESCE(SUM(CASE WHEN kind='RETURN' THEN -amount ELSE 0 END),0),
       COALESCE(SUM(cost),0), COALESCE(SUM(cost IS NULL),0)
FROM events CROSS JOIN scope
WHERE julianday(event_at)>=start_at AND julianday(event_at)<end_at
`
