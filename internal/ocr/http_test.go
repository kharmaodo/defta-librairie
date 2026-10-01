package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func goodResult() map[string]any {
	return map[string]any{"schemaVersion": 1, "engine": "tesseract-experimental", "engineVersion": "tesseract 5.3.0", "policyVersion": "ocr-local-v1", "language": "ara", "textRaw": "كِتاب", "textNormalized": "كتاب", "confidence": 0.8, "psm": 11, "preprocessing": "original"}
}

func TestHTTPMultipartMetadata(t *testing.T) {
	image := testPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/ocr" || r.Header.Get("X-Request-Id") != "job-1" {
			t.Errorf("wrong request contract")
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if !bytes.Equal(data, image) || header.Header.Get("Content-Type") != "image/png" {
			t.Errorf("incorrect multipart image")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(goodResult())
	}))
	defer server.Close()
	runner, err := NewHTTP(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Extract(context.Background(), image, "image/png", "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.TextRaw != "كِتاب" || result.TextNormalized != "كتاب" || result.Confidence == nil || *result.Confidence != 0.8 || result.PSM != 11 || result.EngineVersion != "tesseract 5.3.0" {
		t.Fatalf("incorrect result %#v", result)
	}
}

func TestHTTPResponseValidation(t *testing.T) {
	for _, field := range []string{"schemaVersion", "engine", "engineVersion", "policyVersion", "language", "textRaw", "textNormalized", "confidence", "psm", "preprocessing"} {
		t.Run("missing-"+field, func(t *testing.T) {
			data := goodResult()
			delete(data, field)
			raw, _ := json.Marshal(data)
			if _, err := decodeResult(raw); !errors.Is(err, ErrInvalidResponse) {
				t.Fatal("accepted missing field")
			}
		})
	}
	for _, tc := range []struct {
		field string
		value any
	}{{"schemaVersion", 2}, {"schemaVersion", "1"}, {"engine", "remote-cloud"}, {"engineVersion", "tesseract 4.0"}, {"language", "eng"}, {"policyVersion", "future-policy"}, {"psm", 3}, {"preprocessing", "central-red"}, {"confidence", -0.1}, {"confidence", 1.1}, {"confidence", "0.8"}, {"textRaw", strings.Repeat("x", 1024*1024+1)}, {"textNormalized", 23}, {"unexpected", true}} {
		t.Run(tc.field, func(t *testing.T) {
			data := goodResult()
			data[tc.field] = tc.value
			raw, _ := json.Marshal(data)
			if _, err := decodeResult(raw); !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("accepted invalid %s", tc.field)
			}
		})
	}
	data := goodResult()
	data["confidence"] = nil
	raw, _ := json.Marshal(data)
	if result, err := decodeResult(raw); err != nil || result.Confidence != nil {
		t.Fatal("null confidence not retained")
	}
	for _, raw := range []string{"{}", string(raw) + "{}", "not json", `{"schemaVersion":2,"schemaVersion":1}`} {
		if _, err := decodeResult([]byte(raw)); !errors.Is(err, ErrInvalidResponse) {
			t.Fatal("accepted invalid JSON")
		}
	}
}

func TestHTTPErrorsNoFallbackAndNoPrivateDetails(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
		want   error
	}{{503, "OCR_BUSY", ErrBusy}, {503, "OCR_UNAVAILABLE", ErrUnavailable}, {504, "OCR_TIMEOUT", ErrTimeout}, {422, "INVALID_IMAGE", ErrInvalidInput}, {413, "IMAGE_TOO_LARGE", ErrInvalidInput}, {500, "", ErrUnavailable}, {302, "", ErrUnavailable}} {
		t.Run(http.StatusText(tc.status)+tc.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://example.test/private")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"error":{"code":"`+tc.code+`","message":"PRIVATE IMAGE TEXT"}}`)
			}))
			defer server.Close()
			runner, _ := NewHTTP(server.URL, time.Second)
			_, err := runner.Run(context.Background(), testPNG(t), "image/png")
			if !errors.Is(err, tc.want) || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatalf("wrong safe error: %v", err)
			}
		})
	}
}

func TestHTTPTimeoutCancellationAndInvalidContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }))
	defer server.Close()
	runner, _ := NewHTTP(server.URL, 20*time.Millisecond)
	if _, err := runner.Run(context.Background(), testPNG(t), "image/png"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runner.Run(ctx, testPNG(t), "image/png"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := runner.Run(context.Background(), []byte("bad"), "image/png"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("input: %v", err)
	}
	for _, body := range []string{"{}", strings.Repeat("x", 8*1024*1024+1), "\xff"} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		}))
		h, _ := NewHTTP(s.URL, time.Second)
		_, err := h.Run(context.Background(), testPNG(t), "image/png")
		s.Close()
		if !errors.Is(err, ErrInvalidResponse) {
			t.Fatalf("response: %v", err)
		}
	}
}

func TestHTTPOriginValidationAndProxyDisabled(t *testing.T) {
	for _, endpoint := range []string{"", "file:///secret", "http://example.test/path", "http://user:password@localhost", "http://127.0.0.1?secret=x", "http://8.8.8.8", "http://169.254.169.254", "http://localhost:0", "http://localhost#fragment"} {
		if _, err := NewHTTP(endpoint, time.Second); err == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
	runner, err := NewHTTP("http://ocr-experimental:8091", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if runner.client.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("environment proxy enabled")
	}
	if privateIP([]byte{8, 8, 8, 8}) {
		t.Fatal("public destination allowed")
	}
}

// Executed inside the isolated FastAPI container by CI, using the real HTTP contract.
func TestHTTPContainerIntegration(t *testing.T) {
	endpoint := os.Getenv("OCR_INTEGRATION_ENDPOINT")
	if endpoint == "" {
		t.Skip("requires isolated OCR container")
	}
	runner, err := NewHTTP(endpoint, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.Extract(context.Background(), testPNG(t), "image/png", "go-container-integration")
	if err != nil {
		t.Fatal(err)
	}
	if result.Engine != "tesseract-experimental" || result.PolicyVersion != "ocr-local-v1" || result.Confidence != nil || !strings.HasPrefix(result.EngineVersion, "tesseract 5.") {
		t.Fatalf("invalid real runtime result")
	}
}
