package models

type SaleStatus string

const (
	SaleStatusDraft     SaleStatus = "DRAFT"
	SaleStatusConfirmed SaleStatus = "CONFIRMED"
	SaleStatusCancelled SaleStatus = "CANCELLED"
)

type SaleLine struct {
	ID            string  `json:"id"`
	SaleID        string  `json:"saleId"`
	BookID        int64   `json:"bookId"`
	TitleSnapshot string  `json:"title"`
	Quantity      int     `json:"quantity"`
	UnitPrice     float64 `json:"unitPrice"`
	LineTotal     float64 `json:"lineTotal"`
	CreatedAt     string  `json:"createdAt"`
}

type Sale struct {
	ID           string     `json:"id"`
	LibraryID    string     `json:"libraryId"`
	Reference    string     `json:"reference"`
	CustomerName string     `json:"customerName,omitempty"`
	Status       SaleStatus `json:"status"`
	TotalAmount  float64    `json:"totalAmount"`
	Version      int        `json:"version"`
	CreatedBy    string     `json:"createdBy"`
	ConfirmedBy  string     `json:"confirmedBy,omitempty"`
	CancelledBy  string     `json:"cancelledBy,omitempty"`
	CreatedAt    string     `json:"createdAt"`
	UpdatedAt    string     `json:"updatedAt"`
	ConfirmedAt  string     `json:"confirmedAt,omitempty"`
	CancelledAt  string     `json:"cancelledAt,omitempty"`
	Lines        []SaleLine `json:"lines"`
}

type SaleLineInput struct {
	BookID   int64 `json:"bookId"`
	Quantity int   `json:"quantity"`
}

type SaleInput struct {
	LibraryID    string          `json:"libraryId,omitempty"`
	CustomerName string          `json:"customerName,omitempty"`
	Lines        []SaleLineInput `json:"lines"`
	Version      int             `json:"version,omitempty"`
}
