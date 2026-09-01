//go:build fts5
// +build fts5

// cmd/main.go
package main

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/bootstrap"
	"defta-librairie/internal/config"
	"defta-librairie/internal/database"
	"defta-librairie/internal/handlers"
	"defta-librairie/internal/middleware"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	Tmpl *template.Template // ← Exporté (majuscule) pour être visible depuis handlers
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

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap-admin":
			user, err := bootstrap.RootFromEnvironment(context.Background(), database.DB)
			if err != nil {
				log.Fatalf("Échec bootstrap SUPER_ADMIN_ROOT : %v", err)
			}
			log.Printf("SUPER_ADMIN_ROOT créé → username=%s id=%s", user.Username, user.ID)
			return
		case "reset-root-password":
			user, err := bootstrap.ResetRootPasswordFromEnvironment(context.Background(), database.DB)
			if err != nil {
				log.Fatalf("Échec réinitialisation SUPER_ADMIN_ROOT : %v", err)
			}
			log.Printf("Mot de passe SUPER_ADMIN_ROOT réinitialisé → username=%s id=%s", user.Username, user.ID)
			return
		}
	}

	// Passer la config aux handlers
	handlers.SetConfig(cfg)
	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTAccessTTL)
	if err != nil {
		log.Fatalf("Configuration JWT invalide : %v", err)
	}
	loginService, err := services.NewLoginService(repositories.NewUserRepository(database.DB), tokens)
	if err != nil {
		log.Fatalf("Initialisation authentification impossible : %v", err)
	}
	authHandler := handlers.NewAuthHandler(loginService)
	ownerService := services.NewOwnerService(repositories.NewOwnerRepository(database.DB))
	ownerHandler := handlers.NewOwnerHandler(ownerService)

	// Chargement des templates (déplacé dans handlers)
	handlers.InitTemplates()

	// Routes de base
	mux := http.NewServeMux()
	mux.HandleFunc("/", handlers.CatalogueHandler)
	mux.HandleFunc("/api/books", handlers.APIBooksHandler)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.Handle("GET /api/auth/me", middleware.Authenticate(tokens, http.HandlerFunc(authHandler.Me)))
	rootOnly := func(handler http.Handler) http.Handler {
		return middleware.Authenticate(tokens,
			middleware.RequireRoles(handler, models.RoleSuperAdminRoot))
	}
	mux.Handle("GET /api/admin/owners", rootOnly(http.HandlerFunc(ownerHandler.List)))
	mux.Handle("POST /api/admin/owners", rootOnly(http.HandlerFunc(ownerHandler.Create)))
	mux.Handle("GET /api/admin/owners/{id}", rootOnly(http.HandlerFunc(ownerHandler.Get)))
	mux.Handle("PATCH /api/admin/owners/{id}", rootOnly(http.HandlerFunc(ownerHandler.Update)))
	mux.Handle("DELETE /api/admin/owners/{id}", rootOnly(http.HandlerFunc(ownerHandler.Disable)))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Serveur démarré → http://localhost%s", addr)
	log.Printf("Version %s | Build %s", cfg.Version, cfg.BuildDate)

	server := &http.Server{
		Addr: addr, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Échec démarrage serveur : %v", err)
	}
}
