package unfurl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	text := "Смотри https://example.com/a и http://test.ru/b, а также https://example.com/a (дубль)."
	got := ExtractURLs(text)
	want := []string{"https://example.com/a", "http://test.ru/b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractURLs = %v, want %v", got, want)
	}
}

func TestExtractURLsNoLinks(t *testing.T) {
	if got := ExtractURLs("просто текст без ссылок"); len(got) != 0 {
		t.Fatalf("expected no URLs, got %v", got)
	}
}

func TestFetchParsesOG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head>
			<title>Fallback</title>
			<meta property="og:title" content="Заголовок" />
			<meta property="og:description" content="Описание" />
			<meta property="og:image" content="https://cdn.example.com/img.jpg" />
		</head><body>hi</body></html>`))
	}))
	defer srv.Close()

	p, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if p.Title != "Заголовок" {
		t.Errorf("title = %q, want Заголовок", p.Title)
	}
	if p.Description != "Описание" {
		t.Errorf("description = %q", p.Description)
	}
	if p.Image != "https://cdn.example.com/img.jpg" {
		t.Errorf("image = %q", p.Image)
	}
	if p.URL != srv.URL {
		t.Errorf("url = %q", p.URL)
	}
}

func TestFetchFallsBackToTitleTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Only Title</title></head></html>`))
	}))
	defer srv.Close()

	p, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if p.Title != "Only Title" {
		t.Errorf("title = %q, want Only Title", p.Title)
	}
}

func TestFetchErrorOnHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for 500")
	}
}
