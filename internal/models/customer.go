package models

type CustomerStatus string

const (
	CustomerStatusActive   CustomerStatus = "ACTIVE"
	CustomerStatusDisabled CustomerStatus = "DISABLED"
)

type Customer struct {
	ID        string         `json:"id"`
	LibraryID string         `json:"libraryId"`
	Reference string         `json:"reference"`
	Name      string         `json:"name"`
	Phone     string         `json:"phone,omitempty"`
	Email     string         `json:"email,omitempty"`
	Address   string         `json:"address,omitempty"`
	Notes     string         `json:"notes,omitempty"`
	Status    CustomerStatus `json:"status"`
	Version   int            `json:"version"`
	CreatedBy string         `json:"createdBy"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

type CustomerInput struct {
	LibraryID string `json:"libraryId,omitempty"`
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Address   string `json:"address,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Version   int    `json:"version,omitempty"`
}

type CustomerFilter struct {
	Query  string
	Status CustomerStatus
}
