package ocr

import (
	"strings"
	"testing"
)

func TestMatchingTermsConservativeCorrections(t *testing.T) {
	cases := []struct {
		text       string
		dictionary map[string]int
		want       string
	}{
		{"ضوضاء مجهولة كثيرا كثيرا كثيرا كثيرا كثيرا كثيرا كثيرا كثيرا كثيرا كثيرا كثيرا الْبداية", map[string]int{"البداية": 1}, "البداية"},
		{"البدايه", map[string]int{"البداية": 1}, "البداية"},
		{"البدايه", map[string]int{"البداية": 1, "البدايا": 1}, ""},
		{"كتب", map[string]int{"كتاب": 1}, ""},
		{"NЕAR OR NOT *", map[string]int{"or": 1, "not": 1}, "not or"},
		{"20260", map[string]int{"20261": 1}, ""},
		{"بعيدللغاية", map[string]int{"البداية": 1}, ""},
	}
	for _, c := range cases {
		if got := strings.Join(MatchingTerms(c.text, c.dictionary), " "); got != c.want {
			t.Errorf("%q => %q want %q", c.text, got, c.want)
		}
	}
}
