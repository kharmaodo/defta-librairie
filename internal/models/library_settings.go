package models

type LibrarySettings struct {
	LibraryID                string `json:"libraryId"`
	Name                     string `json:"name"`
	Currency                 string `json:"currency"`
	Address                  string `json:"address"`
	Phone                    string `json:"phone"`
	Email                    string `json:"email"`
	LogoData                 string `json:"logoData"`
	DefaultLowStockThreshold int    `json:"defaultLowStockThreshold"`
	PrintFooter              string `json:"printFooter"`
	Version                  int    `json:"version"`
	UpdatedAt                string `json:"updatedAt"`
}
