// internal/handlers/catalogue.go

package handlers

import (
	"defta-librairie/internal/config"
	"defta-librairie/internal/database"
	"defta-librairie/internal/models"
	"html/template"
	"log"
	"net/http"
)

var (
	tmpl *template.Template // doit être initialisé dans main.go
	cfg  *config.Config
)

func InitTemplates() {
	// À appeler une seule fois dans main.go
	tmpl = template.Must(template.ParseGlob("templates/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("templates/partials/*.html"))
}

func CatalogueHandler(w http.ResponseWriter, r *http.Request) {
	// Récupérer les 30 premiers livres (premier chargement)
	books, total, err := database.SearchBooks("", 0, cfg.PageSize)
	if err != nil {
		log.Printf("Erreur chargement livres initiaux : %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title     string
		Version   string
		BuildDate string
		PageSize  int
		Books     []models.Book
		Total     int
	}{
		Title:     "كتالوج الكتب",
		Version:   cfg.Version,
		BuildDate: cfg.BuildDate,
		PageSize:  cfg.PageSize,
		Books:     books,
		Total:     total,
	}

	// Rendre le template principal
	if err := tmpl.ExecuteTemplate(w, "catalogue.html", data); err != nil {
		log.Printf("Erreur rendu template : %v", err)
		http.Error(w, "Erreur rendu", http.StatusInternalServerError)
	}
}