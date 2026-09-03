package models

type InventoryMovementType string

const (
	InventoryMovementEntry      InventoryMovementType = "ENTRY"
	InventoryMovementExit       InventoryMovementType = "EXIT"
	InventoryMovementAdjustment InventoryMovementType = "ADJUSTMENT"
)

type BookInventory struct {
	BookID            int64  `json:"bookId"`
	LibraryID         string `json:"libraryId"`
	Quantity          int    `json:"quantity"`
	LowStockThreshold int    `json:"lowStockThreshold"`
	Version           int    `json:"version"`
	UpdatedAt         string `json:"updatedAt"`
}

type InventoryMovement struct {
	ID             string                `json:"id"`
	BookID         int64                 `json:"bookId"`
	LibraryID      string                `json:"libraryId"`
	ActorUserID    string                `json:"actorUserId"`
	MovementType   InventoryMovementType `json:"movementType"`
	QuantityDelta  int                   `json:"quantityDelta"`
	QuantityBefore int                   `json:"quantityBefore"`
	QuantityAfter  int                   `json:"quantityAfter"`
	Reason         string                `json:"reason,omitempty"`
	CreatedAt      string                `json:"createdAt"`
}
