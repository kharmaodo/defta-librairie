// cmd/main.go

package main

import (
	"defta-librairie/internal/config"
	"defta-librairie/internal/database"
	"defta-librairie/internal/handlers"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

var (
	tmpl *template.Template
	cfg  *config.Config
)

func main() {
	// Charger la configuration depuis .env
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("Erreur chargement configuration : %v", err)
	}

	// Initialiser la connexion SQLite
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Erreur connexion à la base SQLite : %v", err)
	}
	defer database.Close()

	// ────────────────────────────────────────────────
	// Définir les fonctions personnalisées AVANT ParseGlob
	// ────────────────────────────────────────────────
	funcMap := template.FuncMap{
		"GetMsg": getMsg, // ← la fonction doit exister (voir ci-dessous)
	}

	// Charger les templates AVEC les fonctions
	tmpl = template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseGlob("templates/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("templates/partials/*.html"))

	// Routes de base
	http.HandleFunc("/", handlers.CatalogueHandler)
	http.HandleFunc("/api/books", handlers.APIBooksHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Serveur démarré → http://localhost%s", addr)
	log.Printf("Version %s | Build %s", cfg.Version, cfg.BuildDate)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Échec démarrage serveur : %v", err)
	}
}

// ────────────────────────────────────────────────
// Fonction de traduction minimale (à déplacer plus tard dans internal/i18n)
// ────────────────────────────────────────────────

var translations = map[string]map[string]string{
	"ar": {
		"title":              "كتالوج المكتبة",
		"search_placeholder": "ابحث عن كتاب، مؤلف، دار نشر...",
		"search_button":      "بحث",
		"view_label":         "عرض",
		"view_table":         "جدول",
		"view_cards":         "كروت",
		"no_results":         "لا توجد نتائج مطابقة",
	},
	"fr": {
		"title":              "Catalogue de la librairie",
		"search_placeholder": "Rechercher un livre, auteur, éditeur...",
		"search_button":      "Rechercher",
		"view_label":         "Vue",
		"view_table":         "Tableau",
		"view_cards":         "Cartes",
		"no_results":         "Aucun résultat correspondant",
	},
}

func getMsg(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if val, ok := m[key]; ok {
			return val
		}
	}
	// Fallback arabe
	if m, ok := translations["ar"]; ok {
		if val, ok := m[key]; ok {
			return val
		}
	}
	return key // clé brute si introuvable
}