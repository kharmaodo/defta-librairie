package models

// CustomerSaleSummary preserves the sale's recorded values, including cancelled sales.
type CustomerSaleSummary struct {
	ID          string     `json:"id"`
	Reference   string     `json:"reference"`
	Status      SaleStatus `json:"status"`
	TotalAmount float64    `json:"totalAmount"`
	CreatedAt   string     `json:"createdAt"`
	ConfirmedAt string     `json:"confirmedAt,omitempty"`
	CancelledAt string     `json:"cancelledAt,omitempty"`
}

type CustomerHistoryFilter struct {
	Status SaleStatus
	From   string
	To     string
}
