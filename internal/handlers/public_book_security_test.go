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
	tmpl := template.Must(template.New("x").Parse(`<h3>{{.Title}}</h3>`))
	var out bytes.Buffer
	if err = tmpl.Execute(&out, malicious); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<img") {
		t.Fatal(out.String())
	}
}
