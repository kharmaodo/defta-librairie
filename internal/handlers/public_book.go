package handlers

import (
	"database/sql"
	"defta-librairie/internal/models"
	"strconv"
)

// PublicBook deliberately excludes internal state, FTS scores, library and
// moderation metadata. ID is retained to locate the public READY derivative.
type PublicBook struct {
	ID        int                `json:"id"`
	Title     string             `json:"title"`
	Auteur    models.StringField `json:"auteur"`
	Editeur   models.StringField `json:"editeur"`
	Price     float64            `json:"price"`
	Volume    int                `json:"volume"`
	Tags      models.StringField `json:"tags"`
	Categorie models.StringField `json:"categorie"`
	CoverURL  models.StringField `json:"coverUrl"`
}

func publicBook(b models.Book) PublicBook {
	return PublicBook{ID: b.ID, Title: b.Title, Auteur: b.Auteur, Editeur: b.Editeur, Price: b.Price, Volume: b.Volume, Tags: b.Tags, Categorie: b.Categorie,
		CoverURL: models.StringField{NullString: sql.NullString{String: "/api/books/" + strconv.Itoa(b.ID) + "/cover?variant=large&format=jpeg", Valid: true}}}
}
