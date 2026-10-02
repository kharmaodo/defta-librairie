// internal/handlers/api.go
package handlers

import (
	"defta-librairie/internal/config"
	"defta-librairie/internal/database"
	"defta-librairie/internal/models"

	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

var globalCfg *config.Config

const (
	maxAPIBooksLimit   = 100
	maxAPISearchLength = 256
)

var ErrAPISearchTooLong = errors.New("search query exceeds maximum length")

func SetConfig(c *config.Config) {
	globalCfg = c
}

// cleanBook returns the dedicated public representation.
func cleanBook(b models.Book) PublicBook { return publicBook(b) }

func normalizeAPISearch(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxAPISearchLength {
		return "", ErrAPISearchTooLong
	}
	return value, nil
}

func normalizeAPIPagination(offsetStr, limitStr string, defaultLimit int) (int, int) {
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = defaultLimit
	}
	if limit > maxAPIBooksLimit {
		limit = maxAPIBooksLimit
	}

	return offset, limit
}

func APIBooksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	q, err := normalizeAPISearch(r.URL.Query().Get("q"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_query", "message": "Search query is too long"})
		return
	}
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	defaultLimit := 30
	if globalCfg != nil && globalCfg.PageSize > 0 {
		defaultLimit = globalCfg.PageSize
	}
	offset, limit := normalizeAPIPagination(offsetStr, limitStr, defaultLimit)

	books, total, err := database.SearchBooks(q, offset, limit)
	if err != nil {
		log.Printf("SearchBooks error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "internal_error",
			"message": "Catalogue temporarily unavailable",
		})
		return
	}

	// Nettoyage des résultats avant envoi
	cleanResults := make([]PublicBook, len(books))
	for i, book := range books {
		cleanResults[i] = cleanBook(book)
	}

	resp := struct {
		Results []PublicBook `json:"results"`
		Total   int          `json:"total"`
		Offset  int          `json:"offset"`
		Limit   int          `json:"limit"`
	}{
		Results: cleanResults,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("JSON encode error: %v", err)
		http.Error(w, `{"error":"json encoding failed"}`, http.StatusInternalServerError)
	}
}
