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

	// Chargement des templates (déplacé dans handlers)
	handlers.InitTemplates()

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

