package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/go-chi/chi/v5"
)

// hTestUserRepo — in-memory реализация service.UserRepository для handler-тестов.
type hTestUserRepo struct {
	users    map[string]*models.User
	bySSA    map[string]*models.User
	username map[string]bool
}

func newHTestUserRepo() *hTestUserRepo {
	return &hTestUserRepo{
		users:    map[string]*models.User{},
		bySSA:    map[string]*models.User{},
		username: map[string]bool{},
	}
}

func (m *hTestUserRepo) Create(user *models.User) error {
	if user.ID == "" {
		user.ID = "user-1"
	}
	m.users[user.ID] = user
	m.bySSA[user.SSAID] = user
	return nil
}

func (m *hTestUserRepo) FindBySSAID(ssaID string) (*models.User, error) {
	if u, ok := m.bySSA[ssaID]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestUserRepo) FindByID(id string) (*models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestUserRepo) Update(user *models.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *hTestUserRepo) UpdateLastSeen(userID string) error { return nil }

func (m *hTestUserRepo) UpdateUsername(userID, username string) error {
	if u, ok := m.users[userID]; ok {
		u.Username = username
	}
	return nil
}

func (m *hTestUserRepo) CheckUsername(username string) (bool, error) {
	return m.username[username], nil
}

// hTestSessionRepo — in-memory реализация service.SessionRepository.
type hTestSessionRepo struct {
	sessions map[string]*models.Session
}

func newHTestSessionRepo() *hTestSessionRepo {
	return &hTestSessionRepo{sessions: map[string]*models.Session{}}
}

func (m *hTestSessionRepo) Create(session *models.Session) error {
	if session.ID == "" {
		session.ID = "sess-1"
	}
	m.sessions[session.RefreshToken] = session
	return nil
}

func (m *hTestSessionRepo) FindByRefreshToken(token string) (*models.Session, error) {
	if s, ok := m.sessions[token]; ok {
		return s, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestSessionRepo) Revoke(id string) error {
	for _, s := range m.sessions {
		if s.ID == id {
			s.IsRevoked = true
		}
	}
	return nil
}

func (m *hTestSessionRepo) RevokeByUserID(userID string) error { return nil }

// hTestCacheRepo — in-memory реализация service.CacheRepository.
type hTestCacheRepo struct {
	sessions map[string]*models.Session
	users    map[string]*models.User
}

func newHTestCacheRepo() *hTestCacheRepo {
	return &hTestCacheRepo{sessions: map[string]*models.Session{}, users: map[string]*models.User{}}
}

func (m *hTestCacheRepo) SetSession(_ context.Context, key string, session *models.Session, _ time.Duration) error {
	m.sessions[key] = session
	return nil
}

func (m *hTestCacheRepo) GetSession(_ context.Context, key string) (*models.Session, error) {
	if s, ok := m.sessions[key]; ok {
		return s, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestCacheRepo) DelSession(_ context.Context, key string) error {
	delete(m.sessions, key)
	return nil
}

func (m *hTestCacheRepo) SetUser(_ context.Context, userID string, user *models.User) error {
	m.users[userID] = user
	return nil
}

func (m *hTestCacheRepo) GetUser(_ context.Context, userID string) (*models.User, error) {
	if u, ok := m.users[userID]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestCacheRepo) DelUser(_ context.Context, userID string) error {
	delete(m.users, userID)
	return nil
}

// hTestSSAClient — mock OAuth-провайдера.
type hTestSSAClient struct{}

func (m *hTestSSAClient) AuthorizeURL(state string) string {
	return "https://ssa.test/auth?state=" + state
}

func (m *hTestSSAClient) ExchangeCode(code string) (*repository.SSATokenResponse, error) {
	return &repository.SSATokenResponse{AccessToken: "access-123"}, nil
}

func (m *hTestSSAClient) GetUserInfo(accessToken string) (*repository.SSAUserInfo, error) {
	return &repository.SSAUserInfo{Sub: "ssa-1", Email: "a@b.c", Name: "Alice"}, nil
}

func testJWTConfig() *auth.Config {
	return &auth.Config{
		AccessSecret:  "test-access-secret",
		RefreshSecret: "test-refresh-secret",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}
}

func newAuthRouter(t *testing.T) (http.Handler, *hTestUserRepo, *hTestSessionRepo, *hTestCacheRepo) {
	t.Helper()
	userRepo := newHTestUserRepo()
	sessRepo := newHTestSessionRepo()
	cacheRepo := newHTestCacheRepo()
	svc := service.NewAuthService(userRepo, sessRepo, cacheRepo, &hTestSSAClient{}, testJWTConfig())
	h := NewAuthHandler(svc)
	r := chi.NewRouter()
	r.Get("/authorize", h.SSAAuthorize)
	r.Get("/callback", h.SSACallback)
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/refresh", h.Refresh)
	r.Post("/logout", h.Logout)
	r.Get("/me", h.GetMe)
	r.Patch("/profile", h.UpdateProfile)
	r.Get("/username/{username}", h.CheckUsername)
	r.Patch("/username", h.UpdateUsername)
	return r, userRepo, sessRepo, cacheRepo
}

func TestAuthHandlerSSAAuthorizeRedirect(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/authorize", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" || loc[:7] != "https:/" {
		t.Errorf("expected SSA authorize redirect, got %s", loc)
	}
}

func TestAuthHandlerCallbackMissingParams(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/callback", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerCallbackOk(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=c1&state=s1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w)
	if resp["error"] != false {
		t.Errorf("expected error=false, got %v", resp["error"])
	}
}

func TestAuthHandlerRegisterOk(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"ssa_code":"code1","username":"alice","display_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandlerRegisterMissingSSACode(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerRegisterTooLongUsername(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"ssa_code":"c","username":"` + string(bytes.Repeat([]byte("x"), 33)) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerRegisterUsernameTaken(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.username["taken"] = true

	body := `{"ssa_code":"c","username":"taken"}`
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestAuthHandlerRegisterInvalidBody(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerLoginOk(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"ssa_code":"code1"}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandlerLoginMissingCode(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerRefreshMissingToken(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerRefreshInvalidToken(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"refresh_token":"nope"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandlerRefreshExpiredSession(t *testing.T) {
	router, _, sessRepo, _ := newAuthRouter(t)
	sessRepo.sessions["expired-token"] = &models.Session{
		ID:        "s1",
		IsRevoked: true,
	}

	body := `{"refresh_token":"expired-token"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthHandlerLogout(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"refresh_token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewBufferString(body))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandlerGetMe(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.users["user-1"] = &models.User{ID: "user-1", Username: "alice", DisplayName: "Alice"}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandlerGetMeNotFound(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req = withClaims(req, "ghost")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestAuthHandlerUpdateProfile(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.users["user-1"] = &models.User{ID: "user-1", Username: "alice"}

	body := `{"display_name":"New Name","bio":"hello"}`
	req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBufferString(body))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if userRepo.users["user-1"].DisplayName != "New Name" {
		t.Errorf("expected display_name updated, got %q", userRepo.users["user-1"].DisplayName)
	}
}

func TestAuthHandlerUpdateProfileInvalidBody(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.users["user-1"] = &models.User{ID: "user-1"}

	req := httptest.NewRequest(http.MethodPatch, "/profile", bytes.NewBufferString("{bad"))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerCheckUsername(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.username["busy"] = true

	req := httptest.NewRequest(http.MethodGet, "/username/busy", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["available"] != false {
		t.Errorf("expected available=false, got %v", data["available"])
	}
}

func TestAuthHandlerUpdateUsername(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.users["user-1"] = &models.User{ID: "user-1", Username: "old"}

	body := `{"username":"new"}`
	req := httptest.NewRequest(http.MethodPatch, "/username", bytes.NewBufferString(body))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthHandlerUpdateUsernameEmpty(t *testing.T) {
	router, _, _, _ := newAuthRouter(t)

	body := `{"username":""}`
	req := httptest.NewRequest(http.MethodPatch, "/username", bytes.NewBufferString(body))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandlerUpdateUsernameTaken(t *testing.T) {
	router, userRepo, _, _ := newAuthRouter(t)
	userRepo.users["user-1"] = &models.User{ID: "user-1", Username: "old"}
	userRepo.username["busy"] = true

	body := `{"username":"busy"}`
	req := httptest.NewRequest(http.MethodPatch, "/username", bytes.NewBufferString(body))
	req = withClaims(req, "user-1")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

var _ = json.Marshal
