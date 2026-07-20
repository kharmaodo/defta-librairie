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
	Tmpl *template.Template   // ← Exporté (majuscule) pour être visible depuis handlers
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

	// Passer la config aux handlers
	handlers.SetConfig(cfg)

	// ────────────────────────────────────────────────
	// Définir les fonctions personnalisées AVANT ParseGlob
	// ────────────────────────────────────────────────
	funcMap := template.FuncMap{
		"GetMsg": getMsg,
		"default": func(value, def string) string {
			if value == "" {
				return def
			}
			return value
		},
	}

	// Charger les templates AVEC les fonctions
	Tmpl = template.New("").Funcs(funcMap)
	Tmpl = template.Must(Tmpl.ParseGlob("templates/*.html"))
	Tmpl = template.Must(Tmpl.ParseGlob("templates/partials/*.html"))

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
// Fonction de traduction minimale
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