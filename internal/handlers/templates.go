// internal/handlers/templates.go
package handlers

import (
	"html/template"
	"log"
)

// Tmpl est le template global exporté
var Tmpl *template.Template

func InitTemplates() {
	funcMap := template.FuncMap{
		"GetMsg": getMsg,
		"default": func(value, def string) string {
			if value == "" {
				return def
			}
			return value
		},
	}

	Tmpl = template.New("").Funcs(funcMap)
	var err error
	Tmpl, err = Tmpl.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Erreur parsing templates/*.html : %v", err)
	}
	Tmpl, err = Tmpl.ParseGlob("templates/partials/*.html")
	if err != nil {
		log.Fatalf("Erreur parsing partials : %v", err)
	}

	log.Println("Templates chargés avec succès")
}