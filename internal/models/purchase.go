package models

type PurchaseStatus string

const (
	PurchaseStatusDraft     PurchaseStatus = "DRAFT"
	PurchaseStatusReceived  PurchaseStatus = "RECEIVED"
	PurchaseStatusCancelled PurchaseStatus = "CANCELLED"
)

type PurchaseLine struct {
	ID            string  `json:"id"`
	PurchaseID    string  `json:"purchaseId"`
	BookID        int64   `json:"bookId"`
	TitleSnapshot string  `json:"title"`
	Quantity      int     `json:"quantity"`
	UnitCost      float64 `json:"unitCost"`
	LineTotal     float64 `json:"lineTotal"`
	CreatedAt     string  `json:"createdAt"`
}

type Purchase struct {
	ID          string         `json:"id"`
	LibraryID   string         `json:"libraryId"`
	SupplierID  string         `json:"supplierId"`
	Reference   string         `json:"reference"`
	Status      PurchaseStatus `json:"status"`
	TotalAmount float64        `json:"totalAmount"`
	Version     int            `json:"version"`
	CreatedBy   string         `json:"createdBy"`
	ReceivedBy  string         `json:"receivedBy,omitempty"`
	CancelledBy string         `json:"cancelledBy,omitempty"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	ReceivedAt  string         `json:"receivedAt,omitempty"`
	CancelledAt string         `json:"cancelledAt,omitempty"`
	Lines       []PurchaseLine `json:"lines"`
}

type PurchaseLineInput struct {
	BookID   int64   `json:"bookId"`
	Quantity int     `json:"quantity"`
	UnitCost float64 `json:"unitCost"`
}

type PurchaseInput struct {
	LibraryID  string              `json:"libraryId,omitempty"`
	SupplierID string              `json:"supplierId"`
	Lines      []PurchaseLineInput `json:"lines"`
	Version    int                 `json:"version,omitempty"`
}

type PurchaseFilter struct {
	Status     PurchaseStatus
	SupplierID string
	From       string
	To         string
}
