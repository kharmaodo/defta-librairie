package models

type CustomerReturnStatus string

const (
	CustomerReturnStatusDraft     CustomerReturnStatus = "DRAFT"
	CustomerReturnStatusCompleted CustomerReturnStatus = "COMPLETED"
	CustomerReturnStatusCancelled CustomerReturnStatus = "CANCELLED"
)

type CustomerReturnResolution string

const (
	CustomerReturnResolutionRefund     CustomerReturnResolution = "REFUND"
	CustomerReturnResolutionCreditNote CustomerReturnResolution = "CREDIT_NOTE"
)

type CustomerReturn struct {
	ID          string                   `json:"id"`
	LibraryID   string                   `json:"libraryId"`
	SaleID      string                   `json:"saleId"`
	CustomerID  string                   `json:"customerId,omitempty"`
	Reference   string                   `json:"reference"`
	Reason      string                   `json:"reason"`
	Status      CustomerReturnStatus     `json:"status"`
	Resolution  CustomerReturnResolution `json:"resolution"`
	TotalAmount float64                  `json:"totalAmount"`
	Version     int                      `json:"version"`
	CreatedBy   string                   `json:"createdBy"`
	CompletedBy string                   `json:"completedBy,omitempty"`
	CancelledBy string                   `json:"cancelledBy,omitempty"`
	CreatedAt   string                   `json:"createdAt"`
	UpdatedAt   string                   `json:"updatedAt"`
	CompletedAt string                   `json:"completedAt,omitempty"`
	CancelledAt string                   `json:"cancelledAt,omitempty"`
	Lines       []CustomerReturnLine     `json:"lines"`
}

type CustomerReturnLine struct {
	ID         string  `json:"id"`
	ReturnID   string  `json:"returnId"`
	SaleLineID string  `json:"saleLineId"`
	BookID     int     `json:"bookId"`
	Title      string  `json:"title"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unitPrice"`
	LineTotal  float64 `json:"lineTotal"`
	CreatedAt  string  `json:"createdAt"`
}

type CustomerReturnLineInput struct {
	SaleLineID string `json:"saleLineId"`
	Quantity   int    `json:"quantity"`
}

type CustomerReturnInput struct {
	SaleID     string                   `json:"saleId"`
	Reason     string                   `json:"reason"`
	Resolution CustomerReturnResolution `json:"resolution"`
	Version    int                      `json:"version,omitempty"`
	Lines      []CustomerReturnLineInput `json:"lines"`
}

type ReturnSettlementMethod string

const (
	ReturnSettlementMethodCash        ReturnSettlementMethod = "CASH"
	ReturnSettlementMethodMobileMoney ReturnSettlementMethod = "MOBILE_MONEY"
	ReturnSettlementMethodCard        ReturnSettlementMethod = "CARD"
	ReturnSettlementMethodCreditNote  ReturnSettlementMethod = "CREDIT_NOTE"
)

type ReturnSettlementStatus string

const (
	ReturnSettlementStatusIssued ReturnSettlementStatus = "ISSUED"
	ReturnSettlementStatusVoided ReturnSettlementStatus = "VOIDED"
)

type ReturnSettlement struct {
	ID                string                 `json:"id"`
	LibraryID         string                 `json:"libraryId"`
	ReturnID          string                 `json:"returnId"`
	Method            ReturnSettlementMethod `json:"method"`
	Amount            float64                `json:"amount"`
	ExternalReference string                 `json:"externalReference,omitempty"`
	Notes             string                 `json:"notes,omitempty"`
	Status            ReturnSettlementStatus `json:"status"`
	Version           int                    `json:"version"`
	IssuedBy          string                 `json:"issuedBy"`
	VoidedBy          string                 `json:"voidedBy,omitempty"`
	CreatedAt         string                 `json:"createdAt"`
	UpdatedAt         string                 `json:"updatedAt"`
	VoidedAt          string                 `json:"voidedAt,omitempty"`
}

type CustomerReturnBalance struct {
	ReturnID        string  `json:"returnId"`
	LibraryID       string  `json:"libraryId"`
	TotalAmount     float64 `json:"totalAmount"`
	SettledAmount   float64 `json:"settledAmount"`
	RemainingAmount float64 `json:"remainingAmount"`
	SettlementStatus string `json:"settlementStatus"`
}
