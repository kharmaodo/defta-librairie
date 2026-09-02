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

type ActiveSession struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	Username     string `json:"username"`
	IPAddress    string `json:"ipAddress,omitempty"`
	UserAgent    string `json:"userAgent,omitempty"`
	CreatedAt    string `json:"createdAt"`
	LastUsedAt   string `json:"lastUsedAt,omitempty"`
	ExpiresAt    string `json:"expiresAt"`
}
