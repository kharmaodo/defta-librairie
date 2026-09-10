package handlers

import (
	"bytes"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/csv"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

type CSVExportHandler struct{ service *services.CSVExportService }

func NewCSVExportHandler(s *services.CSVExportService) *CSVExportHandler {
	return &CSVExportHandler{service: s}
}

// Prefix cells which spreadsheet programs may interpret as formulas, including
// formulas preceded by whitespace/control characters. Preserve the original text.
func safeCSVCell(s string) string {
	trimmed := strings.TrimLeftFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\uFEFF' })
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") || strings.HasPrefix(s, "\n") || (trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
		return "'" + s
	}
	return s
}
func (h *CSVExportHandler) Download(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	c, _ := auth.ClaimsFromContext(r.Context())
	if c == nil {
		writeAuthJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	for k, v := range q {
		if len(v) != 1 || !map[string]bool{"libraryId": true, "status": true, "from": true, "to": true, "q": true, "action": true, "resourceType": true}[k] {
			err = repositories.ErrInvalidExport
		}
	}
	if err != nil {
		writeAuthJSON(w, 400, map[string]string{"error": "invalid_export_filter"})
		return
	}
	kind := r.PathValue("kind")
	table, err := h.service.Read(r.Context(), c, kind, repositories.CSVExportFilter{LibraryID: q.Get("libraryId"), Status: q.Get("status"), From: q.Get("from"), To: q.Get("to"), Search: q.Get("q"), Action: q.Get("action"), ResourceType: q.Get("resourceType")})
	if err != nil {
		status, code := 500, "internal_error"
		switch {
		case errors.Is(err, services.ErrBookForbidden):
			status, code = 403, "forbidden"
		case errors.Is(err, repositories.ErrInvalidExport), errors.Is(err, services.ErrInvalidBook):
			status, code = 400, "invalid_export_filter"
		case errors.Is(err, repositories.ErrExportTooLarge):
			status, code = 422, "export_too_large"
		}
		writeAuthJSON(w, status, map[string]string{"error": code})
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'
	writer.UseCRLF = true
	if err = writer.Write(table.Header); err == nil {
		for _, row := range table.Rows {
			for i := range row {
				row[i] = safeCSVCell(row[i])
			}
			if err = writer.Write(row); err != nil {
				break
			}
		}
	}
	writer.Flush()
	if err != nil || writer.Error() != nil {
		writeAuthJSON(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+kind+`.csv"`)
	w.Header().Set("X-Export-Row-Count", strconv.Itoa(len(table.Rows)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
