package handlers

import (
	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverImportErrorClassificationAfterRollback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cause  error
		status int
		code   string
	}{
		{"quota", repositories.ErrCoverImportQuota, http.StatusTooManyRequests, "cover_import_quota_exceeded"},
		{"format", covers.ErrUnsupportedFormat, http.StatusUnprocessableEntity, "invalid_cover_import"},
		{"dimensions", covers.ErrTooManyPixels, http.StatusUnprocessableEntity, "invalid_cover_import"},
		{"size", covers.ErrTooLarge, http.StatusRequestEntityTooLarge, "cover_import_too_large"},
		{"store", covers.ErrStoreUnavailable, http.StatusServiceUnavailable, "cover_import_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := fmt.Errorf("%w: %w", services.ErrCoverPersistence, errors.Join(tc.cause, errors.New("cleanup failed")))
			response := httptest.NewRecorder()
			writeCoverImportError(response, err)
			var payload map[string]string
			if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if response.Code != tc.status || payload["error"] != tc.code {
				t.Fatalf("status=%d payload=%v", response.Code, payload)
			}
		})
	}
}
