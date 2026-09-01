package models

type RefreshSession struct {
	ID           string
	UserID       string
	TokenHash    string
	TokenFamily  string
	ExpiresAt    string
	RevokedAt    string
	ReplacedByID string
	IPAddress    string
	UserAgent    string
	CreatedAt    string
	LastUsedAt   string
}
