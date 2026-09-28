package ocr

import "testing"

func TestNormalizeForMatchingArabicText(t *testing.T) {
	got := NormalizeForMatching("  الْكِتَابُــ: 101!! ")
	if got != "الكتاب 101" { t.Fatalf("got %q", got) }
}

func TestFTSQueryQuotesTokens(t *testing.T) {
	got := FTSQuery("عنوان OR test")
	if got != `"عنوان" AND "OR" AND "test"` { t.Fatalf("got %q", got) }
}
