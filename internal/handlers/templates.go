// internal/handlers/templates.go   (version complète corrigée)

package handlers

import (
	"html/template"
	"log"
)

// ────────────────────────────────────────────────
// Traductions (déplacées ici pour être accessibles aux templates)
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
	return key
}

// Tmpl est le template global exporté
var Tmpl *template.Template

func InitTemplates() {
	funcMap := template.FuncMap{
		"GetMsg":  getMsg,   // ← maintenant visible ici
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
		log.Fatalf("Erreur parsing partials/*.html : %v", err)
	}

	log.Println("Templates chargés avec succès")
}