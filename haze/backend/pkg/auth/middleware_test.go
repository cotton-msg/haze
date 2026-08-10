package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func middlewareTestConfig() *Config {
	return &Config{
		AccessSecret:  "test-secret",
		RefreshSecret: "test-secret",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(ClaimsKey)
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func TestJWTMiddlewareMissingHeader(t *testing.T) {
	handler := JWTMiddleware(middlewareTestConfig())(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without header, got %d", w.Code)
	}
}

func TestJWTMiddlewareInvalidHeaderFormat(t *testing.T) {
	handler := JWTMiddleware(middlewareTestConfig())(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "NotBearer token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad header format, got %d", w.Code)
	}
}

func TestJWTMiddlewareInvalidToken(t *testing.T) {
	handler := JWTMiddleware(middlewareTestConfig())(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad token, got %d", w.Code)
	}
}

func TestJWTMiddlewareValidToken(t *testing.T) {
	cfg := middlewareTestConfig()
	token, err := GenerateAccessToken(cfg, "user-1", "tester", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	var gotClaims *Claims
	handler := JWTMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = r.Context().Value(ClaimsKey).(*Claims)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotClaims == nil || gotClaims.UserID != "user-1" {
		t.Errorf("expected user-1 in claims, got %+v", gotClaims)
	}
}

func TestWSJWTMiddlewareMissingToken(t *testing.T) {
	handler := WSJWTMiddleware(middlewareTestConfig())(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w.Code)
	}
}

func TestWSJWTMiddlewareInvalidToken(t *testing.T) {
	handler := WSJWTMiddleware(middlewareTestConfig())(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/ws?token=garbage", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad token, got %d", w.Code)
	}
}

func TestWSJWTMiddlewareValidToken(t *testing.T) {
	cfg := middlewareTestConfig()
	token, err := GenerateAccessToken(cfg, "user-7", "tester", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	handler := WSJWTMiddleware(cfg)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/ws?token="+token, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
