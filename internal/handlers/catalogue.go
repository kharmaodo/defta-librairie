// internal/handlers/catalogue.go
package handlers

import (
	"defta-librairie/internal/database"
	"defta-librairie/internal/models"
	"log"
	"net/http"
	"strings"   // ← AJOUTE CETTE LIGNE
)

func CatalogueHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("CatalogueHandler - globalCfg = %v", globalCfg)

	q := strings.TrimSpace(r.URL.Query().Get("q"))

	var books []models.Book
	var total int
	var err error

	// Seulement si q n'est PAS vide → on recherche
	if q != "" {
		books, total, err = database.SearchBooks(q, 0, globalCfg.PageSize)
		if err != nil {
			log.Printf("Erreur recherche : %v", err)
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}
	} else {
		// q vide → pas de liste (comportement demandé)
		books = []models.Book{}
		total = 0
	}

	data := struct {
		Title      string
		Lang       string
		Version    string
		BuildDate  string
		PageSize   int
		Query      string
		HasResults bool
		Books      []models.Book
		Total      int
		View       string
	}{
		Title:      "كتالوج الكتب",
		Lang:       "ar",
		Version:    globalCfg.Version,
		BuildDate:  globalCfg.BuildDate,
		PageSize:   globalCfg.PageSize,
		Query:      q,
		HasResults: q != "" && len(books) > 0,
		Books:      books,
		Total:      total,
		View: "card",
	}

	log.Printf("Nombre de livres chargés pour affichage initial : %d", len(books))

	// Utiliser Tmpl global exporté depuis handlers
	if err := Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Erreur rendu template : %v", err)
		http.Error(w, "Erreur lors du rendu du template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}