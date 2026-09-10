package models

type SupplierStatus string

const (
	SupplierStatusActive   SupplierStatus = "ACTIVE"
	SupplierStatusDisabled SupplierStatus = "DISABLED"
)

type Supplier struct {
	ID          string         `json:"id"`
	LibraryID   string         `json:"libraryId"`
	Name        string         `json:"name"`
	ContactName string         `json:"contactName,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Email       string         `json:"email,omitempty"`
	Address     string         `json:"address,omitempty"`
	Status      SupplierStatus `json:"status"`
	Version     int            `json:"version"`
	CreatedBy   string         `json:"createdBy"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
}

type SupplierInput struct {
	LibraryID   string `json:"libraryId,omitempty"`
	Name        string `json:"name"`
	ContactName string `json:"contactName,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Email       string `json:"email,omitempty"`
	Address     string `json:"address,omitempty"`
	Version     int    `json:"version,omitempty"`
}
