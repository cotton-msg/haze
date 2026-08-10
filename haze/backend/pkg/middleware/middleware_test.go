package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDGenerates(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("request ID should be set on the request")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if rid := w.Header().Get("X-Request-ID"); rid == "" {
		t.Error("response should carry X-Request-ID")
	}
}

func TestRequestIDPreservesExisting(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "client-rid-42")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if rid := w.Header().Get("X-Request-ID"); rid != "client-rid-42" {
		t.Errorf("expected existing rid preserved, got %q", rid)
	}
}
