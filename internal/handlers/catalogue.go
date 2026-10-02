// internal/handlers/catalogue.go
package handlers

import (
	"defta-librairie/internal/database"
	"defta-librairie/internal/models"
	"log"
	"net/http"
	"strconv"
)

func CatalogueHandler(w http.ResponseWriter, r *http.Request) {

	q, queryErr := normalizeAPISearch(r.URL.Query().Get("q"))
	if queryErr != nil {
		http.Error(w, "Recherche invalide", http.StatusBadRequest)
		return
	}
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 || page > 1000000 {
		page = 1
	}

	var books []models.Book
	var total int
	var searchErr error

	// Seulement si q n'est PAS vide → on recherche
	if q != "" {
		offset := (page - 1) * globalCfg.PageSize
		books, total, searchErr = database.SearchBooks(q, offset, globalCfg.PageSize)
		if searchErr != nil {
			log.Printf("Erreur recherche : %v", searchErr)
			http.Error(w, "Erreur serveur", http.StatusInternalServerError)
			return
		}

		// Une URL dont la page dépasse le dernier résultat revient à la
		// dernière page valide au lieu d'afficher un faux état vide.
		totalPages := (total + globalCfg.PageSize - 1) / globalCfg.PageSize
		if totalPages > 0 && page > totalPages {
			page = totalPages
			offset = (page - 1) * globalCfg.PageSize
			books, total, searchErr = database.SearchBooks(q, offset, globalCfg.PageSize)
			if searchErr != nil {
				log.Printf("Erreur recherche après correction de page : %v", searchErr)
				http.Error(w, "Erreur serveur", http.StatusInternalServerError)
				return
			}
		}
	} else {
		// q vide → pas de liste (comportement demandé)
		books = []models.Book{}
		total = 0
		page = 1
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + globalCfg.PageSize - 1) / globalCfg.PageSize
	}

	publicResults := make([]PublicBook, len(books))
	for i, b := range books {
		publicResults[i] = publicBook(b)
	}

	data := struct {
		Title      string
		Lang       string
		Version    string
		BuildDate  string
		PageSize   int
		Query      string
		HasResults bool
		Books      []PublicBook
		Total      int
		View       string
		Page       int
		TotalPages int
		HasPrev    bool
		HasNext    bool
		PrevPage   int
		NextPage   int
	}{
		Title:      "كتالوج الكتب",
		Lang:       "ar",
		Version:    globalCfg.Version,
		BuildDate:  globalCfg.BuildDate,
		PageSize:   globalCfg.PageSize,
		Query:      q,
		HasResults: q != "" && len(books) > 0,
		Books:      publicResults,
		Total:      total,
		View:       "card",
		Page:       page,
		TotalPages: totalPages,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
	}

	// Utiliser Tmpl global exporté depuis handlers
	if err := Tmpl.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Print("Catalogue template rendering failed")
		http.Error(w, "Erreur de rendu", http.StatusInternalServerError)
		return
	}
}
