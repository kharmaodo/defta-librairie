// internal/handlers/catalogue.go
package handlers

import (
	"defta-librairie/internal/database"
	"defta-librairie/internal/models"
	"log"
	"net/http"
)

func CatalogueHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("CatalogueHandler - globalCfg = %v", globalCfg)

	// Récupérer les 30 premiers livres (premier chargement)
	books, total, err := database.SearchBooks("", 0, globalCfg.PageSize)
	if err != nil {
		log.Printf("Erreur chargement livres initiaux : %v", err)
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	data := struct {
    Title     string
    Lang      string   // ← AJOUTE ÇA
    Version   string
    BuildDate string
    PageSize  int
    Books     []models.Book
    Total     int
}{
    Title:     "كتالوج الكتب",
    Lang:      "ar",   // ← valeur par défaut arabe + RTL
    Version:   globalCfg.Version,
    BuildDate: globalCfg.BuildDate,
    PageSize:  globalCfg.PageSize,
    Books:     books,
    Total:     total,
}

	// Utiliser Tmpl global exporté depuis handlers
	if err := Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
    log.Printf("ERREUR RENDU BASE : %v", err)
    http.Error(w, "Erreur rendu base : "+err.Error(), 500)
    return
}
}