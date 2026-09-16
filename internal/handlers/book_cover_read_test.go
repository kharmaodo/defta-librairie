package handlers

import (
	"bytes"
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeBookCoverReader struct {
	result  services.ActiveBookCover
	err     error
	bookID  int
	variant string
	format  string
}

func (f *fakeBookCoverReader) Open(
	_ context.Context,
	_ *auth.Claims,
	bookID int,
	variant string,
	format string,
) (services.ActiveBookCover, error) {
	f.bookID, f.variant, f.format = bookID, variant, format
	return f.result, f.err
}

func TestBookCoverReadStreamsPrivateRepresentation(t *testing.T) {
	reader := &fakeBookCoverReader{result: services.ActiveBookCover{
		Body: io.NopCloser(bytes.NewBufferString("webp-cover")),
		ContentType: "image/webp",
		ETag: "\"cover-etag\"",
	}}
	handler := NewBookCoverReadHandler(reader, true)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/manage/books/42/cover?variant=thumb&format=webp",
		nil,
	)
	request.SetPathValue("id", "42")
	response := httptest.NewRecorder()

	handler.Serve(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "webp-cover" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "image/webp" ||
		response.Header().Get("Cache-Control") != "private, max-age=300" ||
		response.Header().Get("ETag") != "\"cover-etag\"" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unexpected headers: %v", response.Header())
	}
	if reader.bookID != 42 || reader.variant != "thumb" || reader.format != "webp" {
		t.Fatalf("unexpected service call: %+v", reader)
	}
}

func TestBookCoverReadHonorsConditionalRequest(t *testing.T) {
	reader := &fakeBookCoverReader{result: services.ActiveBookCover{
		Body: io.NopCloser(bytes.NewBufferString("not-written")),
		ContentType: "image/jpeg",
		ETag: "\"cover-etag\"",
	}}
	handler := NewBookCoverReadHandler(reader, true)
	request := httptest.NewRequest(http.MethodGet, "/api/manage/books/42/cover", nil)
	request.SetPathValue("id", "42")
	request.Header.Set("If-None-Match", "\"cover-etag\"")
	response := httptest.NewRecorder()

	handler.Serve(response, request)

	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if reader.variant != "large" || reader.format != "jpeg" {
		t.Fatalf("unexpected defaults: %+v", reader)
	}
}

func TestBookCoverReadRejectsInvalidRepresentation(t *testing.T) {
	reader := &fakeBookCoverReader{}
	handler := NewBookCoverReadHandler(reader, true)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/manage/books/42/cover?variant=master&format=webp",
		nil,
	)
	request.SetPathValue("id", "42")
	response := httptest.NewRecorder()

	handler.Serve(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
