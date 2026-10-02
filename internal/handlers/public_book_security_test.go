package handlers

import (
	"bytes"
	"database/sql"
	"defta-librairie/internal/models"
	"encoding/json"
	"html/template"
	"strings"
	"testing"
)

func TestPublicBookDoesNotExposeInternalStateAndMapsAuthor(t *testing.T) {
	b := models.Book{ID: 470, Title: "Distinct title", Auteur: models.StringField{NullString: sql.NullString{String: "Distinct author", Valid: true}}, Status: models.StringField{NullString: sql.NullString{String: "MODIFIED", Valid: true}}, Score: sql.NullFloat64{Float64: 123, Valid: true}}
	value := publicBook(b)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if value.Auteur.String == value.Title || strings.Contains(string(data), "MODIFIED") || strings.Contains(string(data), "score") || strings.Contains(string(data), "libraryId") {
		t.Fatal(string(data))
	}
	malicious := publicBook(models.Book{Title: `<img src=x onerror=alert(1)>`})
	tmpl, err := template.New("catalogue").Funcs(template.FuncMap{
		"GetMsg": func(lang, key string) string { return key },
		"default": func(value, fallback string) string {
			if value == "" {
				return fallback
			}
			return value
		},
	}).ParseFiles("../../templates/base.html", "../../templates/catalogue.html")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = tmpl.ExecuteTemplate(&out, "base.html", map[string]interface{}{
		"Title": "Catalogue", "Lang": "ar", "Query": malicious.Title,
		"HasResults": true, "Books": []PublicBook{malicious, value},
		"Total": 2, "PageSize": 30, "View": "card", "TotalPages": 1,
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<img src=x") ||
		strings.Contains(out.String(), "MODIFIED") ||
		!strings.Contains(out.String(), "Distinct author") ||
		!strings.Contains(out.String(), "&lt;img") {
		t.Fatal(out.String())
	}
}
