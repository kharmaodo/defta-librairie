package models

// CatalogueReference is a translated global category or publisher.
type CatalogueReference struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Arabic    string `json:"ar"`
	French    string `json:"fr"`
	English   string `json:"en"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
