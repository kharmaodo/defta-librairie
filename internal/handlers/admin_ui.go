package handlers

import (
	"defta-librairie/internal/config"
	"net/http"
)

type AdminUIHandler struct {
	version   string
	buildDate string
}

type adminUIData struct {
	Title     string
	Version   string
	BuildDate string
}

func NewAdminUIHandler(cfg *config.Config) *AdminUIHandler {
	return &AdminUIHandler{version: cfg.Version, buildDate: cfg.BuildDate}
}

func (h *AdminUIHandler) Login(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "login.html", "Connexion")
}

func (h *AdminUIHandler) Dashboard(w http.ResponseWriter, _ *http.Request) {
	h.render(w, "admin.html", "Administration")
}

func (h *AdminUIHandler) render(w http.ResponseWriter, name, title string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := Tmpl.ExecuteTemplate(w, name, adminUIData{
		Title: title, Version: h.version, BuildDate: h.buildDate,
	}); err != nil {
		http.Error(w, "Erreur de rendu", http.StatusInternalServerError)
	}
}
