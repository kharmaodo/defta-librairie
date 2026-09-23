package moderation

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPClientModerateSendsImageAndParsesDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/moderate" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("content type = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "image" {
			t.Fatalf("body = %q", body)
		}
		_, _ = fmt.Fprint(w, `{"class":"REVIEW","score":0.42,"modelVersion":"model-v1"}`)
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Moderate(context.Background(), "image/jpeg", []byte("image"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != "REVIEW" || got.Score != 0.42 || got.ModelVersion != "model-v1" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestHTTPClientModerateRejectsInvalidResponses(t *testing.T) {
	cases := []string{
		`{"class":"OTHER","score":0.4,"modelVersion":"v1"}`,
		`{"class":"SAFE","score":-0.1,"modelVersion":"v1"}`,
		`{"class":"SAFE","score":0.4,"modelVersion":""}`,
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, body)
			}))
			defer server.Close()
			client, err := NewHTTPClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = client.Moderate(context.Background(), "image/png", []byte("image")); err == nil {
				t.Fatal("expected invalid response error")
			}
		})
	}
}

func TestHTTPClientModerateRejectsBadRequest(t *testing.T) {
	client, err := NewHTTPClient("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Moderate(context.Background(), "image/gif", []byte("image")); err == nil {
		t.Fatal("expected invalid request error")
	}
}
