package handlers

import (
	"bytes"
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
)

type fakeBookCoverUploader struct {
	result      services.PendingBookCover
	err         error
	bookID      int
	contentType string
	data        []byte
}

func (f *fakeBookCoverUploader) Upload(
	_ context.Context,
	_ *auth.Claims,
	bookID int,
	contentType string,
	body io.Reader,
) (services.PendingBookCover, error) {
	f.bookID, f.contentType = bookID, contentType
	f.data, _ = io.ReadAll(body)
	return f.result, f.err
}

func coverRequest(t *testing.T, data []byte, contentType string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="cover"; filename="cover.jpg"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/manage/books/42/cover", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.SetPathValue("id", "42")
	return request
}

func TestBookCoverUploadReturnsAcceptedPendingCover(t *testing.T) {
	service := &fakeBookCoverUploader{result: services.PendingBookCover{
		ID: "cover-id", BookID: 42, LibraryID: "library-id", Status: "PENDING",
		ContentType: "image/jpeg", Format: "jpeg", Width: 4, Height: 5, Size: 8,
	}}
	handler := NewBookCoverHandler(service, true, 1024)
	request := coverRequest(t, []byte("contents"), "image/jpeg")
	response := httptest.NewRecorder()

	handler.Upload(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "/api/manage/books/42/cover/cover-id" {
		t.Fatalf("location=%q", response.Header().Get("Location"))
	}
	if service.bookID != 42 || service.contentType != "image/jpeg" ||
		!bytes.Equal(service.data, []byte("contents")) {
		t.Fatalf("unexpected service call: %+v", service)
	}
}

func TestBookCoverUploadRejectsDisabledFeatureBeforeParsing(t *testing.T) {
	handler := NewBookCoverHandler(nil, false, 1024)
	request := httptest.NewRequest(http.MethodPost, "/api/manage/books/42/cover", bytes.NewBufferString("invalid"))
	request.SetPathValue("id", "42")
	response := httptest.NewRecorder()

	handler.Upload(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBookCoverReplacementRequiresModeration(t *testing.T) {
	response := httptest.NewRecorder()
	NewBookCoverHandler(nil, true, 1024).ModerationRequired(response, httptest.NewRequest(http.MethodPost, "/api/manage/books/42/cover", nil))
	if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte("cover_moderation_required")) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestBookCoverUploadRejectsOversizedFile(t *testing.T) {
	handler := NewBookCoverHandler(&fakeBookCoverUploader{}, true, 4)
	request := coverRequest(t, []byte("too large"), "image/jpeg")
	response := httptest.NewRecorder()

	handler.Upload(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
