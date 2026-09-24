// internal/models/book.go

package models

import (
	"database/sql"
	"encoding/json"
)

// StringField permet un JSON propre : "valeur" ou null
type StringField struct {
	sql.NullString
}

func (sf StringField) MarshalJSON() ([]byte, error) {
	if sf.Valid {
		return json.Marshal(sf.String)
	}
	return json.Marshal(nil)
}

func (sf *StringField) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		sf.String = ""
		sf.Valid = false
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	sf.String = value
	sf.Valid = true
	return nil
}

// IntField pour les champs entiers nullable
type IntField struct {
	sql.NullInt64
}

func (ifld IntField) MarshalJSON() ([]byte, error) {
	if ifld.Valid {
		return json.Marshal(ifld.Int64)
	}
	return json.Marshal(nil)
}

func (ifld *IntField) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		ifld.Int64 = 0
		ifld.Valid = false
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	ifld.Int64 = value
	ifld.Valid = true
	return nil
}

type Book struct {
	ID        int             `json:"id"`
	Title     string          `json:"title"`
	Auteur    StringField     `json:"auteur"`
	Editeur   StringField     `json:"editeur"`
	Price     float64         `json:"price"`
	Volume    int             `json:"volume"`
	Status    StringField     `json:"status"`
	Tags      StringField     `json:"tags"`
	Categorie StringField     `json:"categorie"`
	PublisherID IntField      `json:"publisherId"`
	CategoryIDs []int         `json:"categoryIds,omitempty"`
	PrimaryCategoryID IntField `json:"primaryCategoryId"`
	CoverURL  StringField     `json:"coverUrl"`
	Score     sql.NullFloat64 `json:"score,omitempty"`
	LibraryID string          `json:"libraryId,omitempty"`
	CreatedAt string          `json:"createdAt,omitempty"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
	Version   int             `json:"version,omitempty"`
}

type BookInput struct {
	Title     string  `json:"title"`
	Auteur    string  `json:"auteur"`
	Editeur   string  `json:"editeur"`
	Price     float64 `json:"price"`
	Volume    int     `json:"volume"`
	Status    string  `json:"status"`
	Tags      string  `json:"tags"`
	Categorie string  `json:"categorie"`
	PublisherID *int   `json:"publisherId,omitempty"`
	CategoryIDs []int  `json:"categoryIds,omitempty"`
	PrimaryCategoryID *int `json:"primaryCategoryId,omitempty"`
	CoverURL  string  `json:"coverUrl"`
	LibraryID string  `json:"libraryId,omitempty"`
	Version   int     `json:"version,omitempty"`
}
