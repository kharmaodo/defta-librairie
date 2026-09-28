package models

// CoverImport is a batch of untrusted cover files. It never creates a book.
type CoverImport struct {
	ID            string           `json:"id"`
	LibraryID     string           `json:"libraryId"`
	Status        string           `json:"status"`
	TotalFiles    int              `json:"totalFiles"`
	AcceptedFiles int              `json:"acceptedFiles"`
	RejectedFiles int              `json:"rejectedFiles"`
	CreatedAt     string           `json:"createdAt"`
	UpdatedAt     string           `json:"updatedAt"`
	CompletedAt   *string          `json:"completedAt,omitempty"`
	Jobs          []CoverImportJob `json:"jobs,omitempty"`
}

type CoverImportJob struct {
	ID              string `json:"id"`
	ImportID        string `json:"importId"`
	LibraryID       string `json:"libraryId"`
	Status          string `json:"status"`
	ContentType     string `json:"contentType"`
	Format          string `json:"format"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Size            int64  `json:"size"`
	NSFWDecision    string `json:"nsfwDecision,omitempty"`
	DecisionCode    string `json:"decisionCode,omitempty"`
	FailureCode     string `json:"failureCode,omitempty"`
	TargetBookID    *int   `json:"targetBookId,omitempty"`
	ExpiresAt       string `json:"expiresAt"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}
