package repositories

import "database/sql"

// receiptAverageCost preserves an unknown opening valuation until stock is depleted.
func receiptAverageCost(before int, previous sql.NullFloat64, received int, unitCost float64) sql.NullFloat64 {
	if before == 0 { return sql.NullFloat64{Float64: unitCost, Valid: true} }
	if !previous.Valid { return sql.NullFloat64{} }
	// Weighted terms avoid an intermediate quantity * cost overflow.
	weight := float64(received) / (float64(before) + float64(received))
	return sql.NullFloat64{Float64: previous.Float64*(1-weight) + unitCost*weight, Valid: true}
}
