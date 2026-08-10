package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
)

type mockUserRepo struct {
	users    map[string]*models.User
	bySSA    map[string]*models.User
	created  []*models.User
	username map[string]bool
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:    map[string]*models.User{},
		bySSA:    map[string]*models.User{},
		username: map[string]bool{},
	}
}

func (m *mockUserRepo) Create(user *models.User) error {
	if user.ID == "" {
		user.ID = "user-1"
	}
	m.created = append(m.created, user)
	m.users[user.ID] = user
	m.bySSA[user.SSAID] = user
	return nil
}

func (m *mockUserRepo) FindBySSAID(ssaID string) (*models.User, error) {
	if u, ok := m.bySSA[ssaID]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) FindByID(id string) (*models.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) Update(user *models.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) UpdateLastSeen(userID string) error {
	return nil
}

func (m *mockUserRepo) UpdateUsername(userID, username string) error {
	if u, ok := m.users[userID]; ok {
		u.Username = username
	}
	return nil
}

func (m *mockUserRepo) CheckUsername(username string) (bool, error) {
	return m.username[username], nil
}

type mockSessionRepo struct {
	sessions map[string]*models.Session // by refresh token
	revoked  []string
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{sessions: map[string]*models.Session{}}
}

func (m *mockSessionRepo) Create(session *models.Session) error {
	if session.ID == "" {
		session.ID = "sess-1"
	}
	m.sessions[session.RefreshToken] = session
	return nil
}

func (m *mockSessionRepo) FindByRefreshToken(token string) (*models.Session, error) {
	if s, ok := m.sessions[token]; ok {
		return s, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockSessionRepo) Revoke(id string) error {
	m.revoked = append(m.revoked, id)
	for _, s := range m.sessions {
		if s.ID == id {
			s.IsRevoked = true
		}
	}
	return nil
}

func (m *mockSessionRepo) RevokeByUserID(userID string) error {
	return nil
}

type mockCacheRepo struct {
	sessions map[string]*models.Session
	users    map[string]*models.User
}

func newMockCacheRepo() *mockCacheRepo {
	return &mockCacheRepo{sessions: map[string]*models.Session{}, users: map[string]*models.User{}}
}

func (m *mockCacheRepo) SetSession(ctx context.Context, key string, session *models.Session, ttl time.Duration) error {
	m.sessions[key] = session
	return nil
}

func (m *mockCacheRepo) GetSession(ctx context.Context, key string) (*models.Session, error) {
	if s, ok := m.sessions[key]; ok {
		return s, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockCacheRepo) DelSession(ctx context.Context, key string) error {
	delete(m.sessions, key)
	return nil
}

func (m *mockCacheRepo) SetUser(ctx context.Context, userID string, user *models.User) error {
	m.users[userID] = user
	return nil
}

func (m *mockCacheRepo) GetUser(ctx context.Context, userID string) (*models.User, error) {
	if u, ok := m.users[userID]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockCacheRepo) DelUser(ctx context.Context, userID string) error {
	delete(m.users, userID)
	return nil
}

// mockSSAClient эмулирует OAuth-провайдера.
type mockSSAClient struct {
	userInfo *repository.SSAUserInfo
	token    *repository.SSATokenResponse
}

func (m *mockSSAClient) AuthorizeURL(state string) string {
	return "https://ssa.test/auth?state=" + state
}

func (m *mockSSAClient) ExchangeCode(code string) (*repository.SSATokenResponse, error) {
	if m.token != nil {
		return m.token, nil
	}
	return &repository.SSATokenResponse{AccessToken: "access-123"}, nil
}

func (m *mockSSAClient) GetUserInfo(accessToken string) (*repository.SSAUserInfo, error) {
	if m.userInfo != nil {
		return m.userInfo, nil
	}
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

func newTestAuthService(user *mockUserRepo, sess *mockSessionRepo, cache *mockCacheRepo, ssa *mockSSAClient) *AuthService {
	return NewAuthService(user, sess, cache, ssa, testJWTConfig())
}

func TestGenerateState(t *testing.T) {
	state1 := GenerateState()
	state2 := GenerateState()

	if state1 == "" {
		t.Fatal("state should not be empty")
	}
	if state1 == state2 {
		t.Fatal("consecutive states should be different")
	}
}

func TestErrorsAreDefined(t *testing.T) {
	if ErrUserNotFound == nil {
		t.Fatal("ErrUserNotFound should be defined")
	}
	if ErrInvalidToken == nil {
		t.Fatal("ErrInvalidToken should be defined")
	}
	if ErrSessionExpired == nil {
		t.Fatal("ErrSessionExpired should be defined")
	}
	if ErrUsernameTaken == nil {
		t.Fatal("ErrUsernameTaken should be defined")
	}
}

func TestHandleSSACallbackCreatesUser(t *testing.T) {
	userRepo := newMockUserRepo()
	sessRepo := newMockSessionRepo()
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	tokens, user, err := svc.HandleSSACallback(context.Background(), "code-1", "UA", "127.0.0.1")
	if err != nil {
		t.Fatalf("HandleSSACallback: %v", err)
	}
	if user.SSAID != "ssa-1" {
		t.Errorf("expected ssa-1, got %s", user.SSAID)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if len(userRepo.created) != 1 {
		t.Fatalf("expected 1 created user, got %d", len(userRepo.created))
	}
}

func TestHandleSSACallbackExistingUser(t *testing.T) {
	userRepo := newMockUserRepo()
	existing := &models.User{ID: "user-existing", SSAID: "ssa-1", DisplayName: "Alice"}
	userRepo.users[existing.ID] = existing
	userRepo.bySSA[existing.SSAID] = existing
	sessRepo := newMockSessionRepo()
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	_, user, err := svc.HandleSSACallback(context.Background(), "code-1", "UA", "ip")
	if err != nil {
		t.Fatalf("HandleSSACallback: %v", err)
	}
	if user.ID != "user-existing" {
		t.Fatalf("expected existing user, got %s", user.ID)
	}
	if len(userRepo.created) != 0 {
		t.Fatalf("expected no new user, got %d", len(userRepo.created))
	}
}

func TestRegisterUsernameTaken(t *testing.T) {
	userRepo := newMockUserRepo()
	userRepo.username["taken"] = true
	sessRepo := newMockSessionRepo()
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	_, _, err := svc.Register(context.Background(), "code-1", "taken", "Display", "UA", "ip")
	if !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestRefreshWithValidSession(t *testing.T) {
	userRepo := newMockUserRepo()
	userRepo.users["user-1"] = &models.User{ID: "user-1", SSAID: "ssa-1", Username: "alice"}
	sessRepo := newMockSessionRepo()
	now := time.Now()
	sessRepo.sessions["refresh-token-1"] = &models.Session{
		ID: "sess-1", UserID: "user-1", RefreshToken: "refresh-token-1",
		ExpiresAt: now.Add(time.Hour), IsRevoked: false,
	}
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	tokens, err := svc.Refresh(context.Background(), "refresh-token-1")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Fatal("expected new access token")
	}
}

func TestRefreshExpiredSessionRejected(t *testing.T) {
	userRepo := newMockUserRepo()
	sessRepo := newMockSessionRepo()
	sessRepo.sessions["refresh-token-1"] = &models.Session{
		ID: "sess-1", UserID: "user-1", RefreshToken: "refresh-token-1",
		ExpiresAt: time.Now().Add(-time.Hour), IsRevoked: false,
	}
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	_, err := svc.Refresh(context.Background(), "refresh-token-1")
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestRefreshUnknownTokenRejected(t *testing.T) {
	userRepo := newMockUserRepo()
	sessRepo := newMockSessionRepo()
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	_, err := svc.Refresh(context.Background(), "nope")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestGetMeReturnsCachedUser(t *testing.T) {
	userRepo := newMockUserRepo()
	cacheRepo := newMockCacheRepo()
	cacheRepo.users["user-1"] = &models.User{ID: "user-1", DisplayName: "Cached"}
	sessRepo := newMockSessionRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	user, err := svc.GetMe(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if user.DisplayName != "Cached" {
		t.Errorf("expected cached user, got %s", user.DisplayName)
	}
}

func TestUpdateUserPreservesUnknownFields(t *testing.T) {
	userRepo := newMockUserRepo()
	userRepo.users["user-1"] = &models.User{ID: "user-1", DisplayName: "Old", Bio: "keep", Username: "alice"}
	sessRepo := newMockSessionRepo()
	cacheRepo := newMockCacheRepo()
	ssa := &mockSSAClient{}
	svc := newTestAuthService(userRepo, sessRepo, cacheRepo, ssa)

	user, err := svc.UpdateUser(context.Background(), "user-1", map[string]interface{}{"display_name": "New"})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if user.DisplayName != "New" {
		t.Errorf("expected New, got %s", user.DisplayName)
	}
	if user.Bio != "keep" {
		t.Errorf("expected Bio preserved, got %s", user.Bio)
	}
}
