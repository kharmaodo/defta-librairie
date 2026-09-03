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
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	userRepository := repositories.NewUserRepository(database.DB)
	loginService, err := services.NewLoginService(userRepository, tokens)
	if err != nil {
		log.Fatalf("Initialisation authentification impossible : %v", err)
	}
	sessionRepository := repositories.NewSessionRepository(database.DB)
	sessionService, err := services.NewSessionService(sessionRepository, tokens, cfg.JWTRefreshTTL)
	if err != nil {
		log.Fatalf("Initialisation sessions impossible : %v", err)
	}
	passwordService := services.NewPasswordService(userRepository)
	authHandler := handlers.NewAuthHandler(loginService, sessionService, passwordService, cfg.AuthCookieSecure)
	ownerService := services.NewOwnerService(repositories.NewOwnerRepository(database.DB))
	ownerHandler := handlers.NewOwnerHandler(ownerService)
	bookService := services.NewBookService(repositories.NewBookRepository(database.DB))
	bookHandler := handlers.NewBookManagementHandler(bookService)
	inventoryService := services.NewInventoryService(repositories.NewInventoryRepository(database.DB))
	inventoryHandler := handlers.NewInventoryHandler(inventoryService)
	tagService := services.NewTagService(repositories.NewTagRepository(database.DB))
	tagHandler := handlers.NewTagHandler(tagService)
	auditService := services.NewAuditService(repositories.NewAuditRepository(database.DB))
	auditHandler := handlers.NewAuditHandler(auditService)
	adminUIHandler := handlers.NewAdminUIHandler(cfg)
	authRateLimiter, err := middleware.NewRateLimiter(cfg.AuthRateLimit, cfg.AuthRateWindow)
	if err != nil {
		log.Fatalf("Configuration rate limit invalide : %v", err)
	}

	// Chargement des templates (déplacé dans handlers)
	handlers.InitTemplates()

	// Routes de base
	mux := http.NewServeMux()
	registerPublicRoutes(mux, adminUIHandler)
	mux.Handle("POST /api/auth/login", authRateLimiter.Limit(http.HandlerFunc(authHandler.Login)))
	mux.Handle("POST /api/auth/refresh", authRateLimiter.Limit(http.HandlerFunc(authHandler.Refresh)))
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	authenticated := func(handler http.Handler) http.Handler {
		return middleware.AuthenticateSession(tokens, sessionRepository, handler)
	}
	mux.Handle("GET /api/auth/me", authenticated(http.HandlerFunc(authHandler.Me)))
	mux.Handle("POST /api/auth/change-password", authenticated(http.HandlerFunc(authHandler.ChangePassword)))
	mux.Handle("GET /api/auth/sessions", authenticated(http.HandlerFunc(authHandler.ActiveSessions)))
	mux.Handle("POST /api/auth/sessions/revoke-others", authenticated(http.HandlerFunc(authHandler.RevokeOtherSessions)))
	mux.Handle("DELETE /api/auth/sessions/{id}", authenticated(http.HandlerFunc(authHandler.RevokeSession)))
	passwordChanged := func(handler http.Handler) http.Handler {
		return authenticated(middleware.RequirePasswordChanged(handler))
	}
	mux.Handle("GET /api/audit-logs", passwordChanged(http.HandlerFunc(auditHandler.List)))
	rootOnly := func(handler http.Handler) http.Handler {
		return passwordChanged(middleware.RequireRoles(handler, models.RoleSuperAdminRoot))
	}
	mux.Handle("GET /api/admin/owners", rootOnly(http.HandlerFunc(ownerHandler.List)))
	mux.Handle("POST /api/admin/owners", rootOnly(http.HandlerFunc(ownerHandler.Create)))
	mux.Handle("GET /api/admin/owners/{id}", rootOnly(http.HandlerFunc(ownerHandler.Get)))
	mux.Handle("PATCH /api/admin/owners/{id}", rootOnly(http.HandlerFunc(ownerHandler.Update)))
	mux.Handle("DELETE /api/admin/owners/{id}", rootOnly(http.HandlerFunc(ownerHandler.Disable)))
	mux.Handle("POST /api/admin/owners/{id}/unlock", rootOnly(http.HandlerFunc(ownerHandler.Unlock)))
	mux.Handle("POST /api/admin/owners/{id}/reactivate", rootOnly(http.HandlerFunc(ownerHandler.Reactivate)))
	mux.Handle("POST /api/admin/owners/{id}/reset-password", rootOnly(http.HandlerFunc(ownerHandler.ResetPassword)))
	bookManagers := func(handler http.Handler) http.Handler {
		return passwordChanged(middleware.RequireRoles(handler,
			models.RoleSuperAdminRoot, models.RoleOwnerLibrary))
	}
	mux.Handle("GET /api/manage/books", bookManagers(http.HandlerFunc(bookHandler.List)))
	mux.Handle("POST /api/manage/books", bookManagers(http.HandlerFunc(bookHandler.Create)))
	mux.Handle("GET /api/manage/books/{id}", bookManagers(http.HandlerFunc(bookHandler.Get)))
	mux.Handle("GET /api/manage/books/{id}/history", bookManagers(http.HandlerFunc(bookHandler.History)))
	mux.Handle("PUT /api/manage/books/{id}", bookManagers(http.HandlerFunc(bookHandler.Update)))
	mux.Handle("DELETE /api/manage/books/{id}", bookManagers(http.HandlerFunc(bookHandler.Delete)))
	mux.Handle("GET /api/manage/inventory", bookManagers(http.HandlerFunc(inventoryHandler.List)))
	mux.Handle("GET /api/manage/books/{id}/inventory", bookManagers(http.HandlerFunc(inventoryHandler.Get)))
	mux.Handle("POST /api/manage/books/{id}/inventory/entries", bookManagers(http.HandlerFunc(inventoryHandler.Entry)))
	mux.Handle("POST /api/manage/books/{id}/inventory/exits", bookManagers(http.HandlerFunc(inventoryHandler.Exit)))
	mux.Handle("PUT /api/manage/books/{id}/inventory", bookManagers(http.HandlerFunc(inventoryHandler.Adjust)))
	mux.Handle("PATCH /api/manage/books/{id}/inventory/threshold", bookManagers(http.HandlerFunc(inventoryHandler.UpdateThreshold)))
	mux.Handle("GET /api/manage/books/{id}/inventory/movements", bookManagers(http.HandlerFunc(inventoryHandler.ListMovements)))
	mux.Handle("GET /api/manage/tags", bookManagers(http.HandlerFunc(tagHandler.List)))
	mux.Handle("POST /api/manage/tags", bookManagers(http.HandlerFunc(tagHandler.Create)))
	mux.Handle("PATCH /api/manage/tags/{id}", bookManagers(http.HandlerFunc(tagHandler.Update)))
	mux.Handle("DELETE /api/manage/tags/{id}", bookManagers(http.HandlerFunc(tagHandler.Delete)))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Serveur démarré → http://localhost%s", addr)
	log.Printf("Version %s | Build %s", cfg.Version, cfg.BuildDate)

	server := &http.Server{
		Addr: addr, Handler: middleware.SecureHTTP(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Échec serveur : %v", err)
		}
	case <-signalContext.Done():
		log.Println("Arrêt gracieux du serveur...")
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdownContext); err != nil {
			log.Printf("Arrêt forcé après échec du shutdown : %v", err)
		}
	}
}

func registerPublicRoutes(mux *http.ServeMux, adminUIHandler *handlers.AdminUIHandler) {
	mux.HandleFunc("GET /{$}", handlers.CatalogueHandler)
	mux.HandleFunc("GET /login", adminUIHandler.Login)
	mux.HandleFunc("GET /admin", adminUIHandler.Dashboard)
	mux.HandleFunc("GET /api/books", handlers.APIBooksHandler)
}
