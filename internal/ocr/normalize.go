package ocr

import (
	"strings"
	"unicode"
)

// NormalizeForMatching produces a deterministic FTS query from OCR text. It
// keeps Arabic letters and digits, removes tashkeel/tatweel, and never sends
// user supplied operators to SQLite FTS5.
func NormalizeForMatching(raw string) string {
	var builder strings.Builder
	space := true
	for _, r := range raw {
		switch {
		case r == '\u0640':
			continue
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			space = false
		default:
			if !space {
				builder.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

// FTSQuery quotes each OCR token so FTS5 operators cannot alter matching.
func FTSQuery(normalized string) string {
	fields := strings.Fields(normalized)
	if len(fields) == 0 { return "" }
	quoted := make([]string, 0, len(fields))
	for _, field := range fields { quoted = append(quoted, `"`+strings.ReplaceAll(field, `"`, ``)+`"`) }
	return strings.Join(quoted, " AND ")
}
