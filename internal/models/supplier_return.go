package models

type SupplierReturnStatus string

const (
	SupplierReturnStatusDraft     SupplierReturnStatus = "DRAFT"
	SupplierReturnStatusShipped   SupplierReturnStatus = "SHIPPED"
	SupplierReturnStatusCancelled SupplierReturnStatus = "CANCELLED"
)

type SupplierReturnLine struct {
	ID             string  `json:"id"`
	ReturnID       string  `json:"returnId"`
	PurchaseLineID string  `json:"purchaseLineId"`
	BookID         int64   `json:"bookId"`
	Title          string  `json:"title"`
	Quantity       int     `json:"quantity"`
	UnitCost       float64 `json:"unitCost"`
	LineTotal      float64 `json:"lineTotal"`
	CreatedAt      string  `json:"createdAt"`
}

type SupplierReturn struct {
	ID                string               `json:"id"`
	LibraryID         string               `json:"libraryId"`
	PurchaseID        string               `json:"purchaseId"`
	SupplierID        string               `json:"supplierId"`
	Reference         string               `json:"reference"`
	SupplierReference string               `json:"supplierReference,omitempty"`
	Reason            string               `json:"reason"`
	Status            SupplierReturnStatus `json:"status"`
	TotalAmount       float64              `json:"totalAmount"`
	Version           int                  `json:"version"`
	CreatedBy         string               `json:"createdBy"`
	ShippedBy         string               `json:"shippedBy,omitempty"`
	CancelledBy       string               `json:"cancelledBy,omitempty"`
	CreatedAt         string               `json:"createdAt"`
	UpdatedAt         string               `json:"updatedAt"`
	ShippedAt         string               `json:"shippedAt,omitempty"`
	CancelledAt       string               `json:"cancelledAt,omitempty"`
	Lines             []SupplierReturnLine `json:"lines"`
}

type SupplierReturnLineInput struct {
	PurchaseLineID string `json:"purchaseLineId"`
	Quantity       int    `json:"quantity"`
}

type SupplierReturnInput struct {
	LibraryID         string                    `json:"libraryId,omitempty"`
	PurchaseID        string                    `json:"purchaseId"`
	SupplierReference string                    `json:"supplierReference,omitempty"`
	Reason            string                    `json:"reason"`
	Version           int                       `json:"version,omitempty"`
	Lines             []SupplierReturnLineInput `json:"lines"`
}

type SupplierReturnFilter struct {
	Status     SupplierReturnStatus
	PurchaseID string
	SupplierID string
	From       string
	To         string
}
