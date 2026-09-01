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
	CoverURL  StringField     `json:"coverUrl"`
	Score     sql.NullFloat64 `json:"score,omitempty"`
}
