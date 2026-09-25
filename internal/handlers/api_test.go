package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAPIPagination(t *testing.T) {
	tests := []struct {
		name         string
		offset       string
		limit        string
		defaultLimit int
		wantOffset   int
		wantLimit    int
	}{
		{name: "valid", offset: "20", limit: "10", defaultLimit: 30, wantOffset: 20, wantLimit: 10},
		{name: "defaults", offset: "", limit: "", defaultLimit: 30, wantOffset: 0, wantLimit: 30},
		{name: "invalid", offset: "abc", limit: "abc", defaultLimit: 25, wantOffset: 0, wantLimit: 25},
		{name: "negative", offset: "-1", limit: "-5", defaultLimit: 30, wantOffset: 0, wantLimit: 30},
		{name: "limit capped", offset: "0", limit: "500", defaultLimit: 30, wantOffset: 0, wantLimit: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, limit := normalizeAPIPagination(tt.offset, tt.limit, tt.defaultLimit)
			if offset != tt.wantOffset || limit != tt.wantLimit {
				t.Fatalf("got offset=%d limit=%d, want offset=%d limit=%d", offset, limit, tt.wantOffset, tt.wantLimit)
			}
		})
	}
}


func TestNormalizeAPISearch(t *testing.T) {
	search, err := normalizeAPISearch("  كتاب  ")
	if err != nil || search != "كتاب" {
		t.Fatalf("search=%q err=%v", search, err)
	}
	if _, err := normalizeAPISearch(strings.Repeat("a", maxAPISearchLength+1)); err == nil {
		t.Fatal("expected oversized search to be rejected")
	}
	if search, err := normalizeAPISearch(strings.Repeat("أ", maxAPISearchLength)); err != nil || len([]rune(search)) != maxAPISearchLength {
		t.Fatalf("expected %d-rune search to be accepted: len=%d err=%v", maxAPISearchLength, len([]rune(search)), err)
	}
}


func TestCatalogueHandlersRejectOversizedSearchBeforeDatabaseAccess(t *testing.T) {
	query := strings.Repeat("a", maxAPISearchLength+1)
	for name, handler := range map[string]http.HandlerFunc{
		"public": APIBooksHandler,
		"managed": NewBookManagementHandler(nil).List,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/books?q="+query, nil)
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d want=%d", response.Code, http.StatusBadRequest)
			}
			var body map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] != "invalid_query" {
				t.Fatalf("body=%v", body)
			}
		})
	}
}
