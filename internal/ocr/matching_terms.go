package ocr

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxMatchingTerms = 128

// MatchingTerms consults only a library dictionary. Unknown words are corrected
// by one edit only when a single sufficiently long dictionary word qualifies.
func MatchingTerms(text string, dictionary map[string]int) []string {
	seen := map[string]bool{}
	words := strings.Fields(strings.ToLower(NormalizeForMatching(text)))
	unknown := map[string]bool{}
	for _, word := range words {
		length := utf8.RuneCountInString(word)
		if length < 2 || length > 64 {
			continue
		}
		if _, ok := dictionary[word]; ok {
			seen[word] = true
		} else if length >= 5 && strings.IndexFunc(word, unicode.IsDigit) < 0 {
			unknown[word] = true
		}
	}
	// Exact evidence covers the entire OCR. Fuzzy work is explicitly bounded.
	misspellings := make([]string, 0, len(unknown))
	for word := range unknown {
		misspellings = append(misspellings, word)
	}
	sort.Strings(misspellings)
	if len(misspellings) > 32 {
		misspellings = misspellings[:32]
	}
	for _, word := range misspellings {
		length := utf8.RuneCountInString(word)
		match := ""
		for candidate := range dictionary {
			n := utf8.RuneCountInString(candidate)
			if n < 5 || n < length-1 || n > length+1 || !oneEdit(word, candidate) {
				continue
			}
			if match != "" {
				match = ""
				break
			}
			match = candidate
		}
		if match != "" {
			seen[match] = true
		}
	}
	terms := make([]string, 0, len(seen))
	for word := range seen {
		terms = append(terms, word)
	}
	// Prefer rare catalogue words when the bounded query cannot carry all evidence.
	sort.Slice(terms, func(i, j int) bool {
		if dictionary[terms[i]] != dictionary[terms[j]] {
			return dictionary[terms[i]] < dictionary[terms[j]]
		}
		return terms[i] < terms[j]
	})
	if len(terms) > MaxMatchingTerms {
		terms = terms[:MaxMatchingTerms]
	}
	return terms
}

func oneEdit(a, b string) bool {
	x, y := []rune(a), []rune(b)
	if len(x) > len(y) {
		x, y = y, x
	}
	if len(y)-len(x) > 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < len(x) && j < len(y) {
		if x[i] == y[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		if len(x) == len(y) {
			i++
		}
		j++
	}
	if i < len(x) || j < len(y) {
		edits++
	}
	return edits == 1
}
