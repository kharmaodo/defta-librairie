package models

// BookSubmission stores a quarantined book creation request. It is not a book
// and must not be exposed by catalogue or managed-book queries.
type BookSubmission struct {
	ID                      string   `json:"id"`
	LibraryID               string   `json:"libraryId"`
	Title                   string   `json:"title"`
	Auteur                  string   `json:"auteur"`
	Editeur                 string   `json:"editeur"`
	Price                   float64  `json:"price"`
	Volume                  int      `json:"volume"`
	Status                  string   `json:"status"`
	Tags                    string   `json:"tags"`
	Categorie               string   `json:"categorie"`
	CoverURL                string   `json:"coverUrl"`
	SourceObjectKey         string   `json:"-"`
	SourceContentType       string   `json:"sourceContentType"`
	SourceFormat            string   `json:"sourceFormat"`
	SourceWidth             int      `json:"sourceWidth"`
	SourceHeight            int      `json:"sourceHeight"`
	SourceSize              int64    `json:"sourceSize"`
	ModerationStatus        string   `json:"moderationStatus"`
	ModerationScore         *float64 `json:"moderationScore,omitempty"`
	ModerationModelVersion  string   `json:"moderationModelVersion,omitempty"`
	DecisionCode            string   `json:"decisionCode,omitempty"`
	CreatedBookID           *int     `json:"createdBookId,omitempty"`
	ExpiresAt               string   `json:"expiresAt"`
	CreatedAt               string   `json:"createdAt"`
	UpdatedAt               string   `json:"updatedAt"`
}
