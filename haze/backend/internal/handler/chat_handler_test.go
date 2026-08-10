package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/go-chi/chi/v5"
)

// hTestChatRepo — in-memory реализация service.ChatRepository для handler-тестов.
type hTestChatRepo struct {
	chats      map[string]*models.Chat
	members    map[string][]string
	memberRole map[string]map[string]models.ChatMemberRole
}

func newHTestChatRepo() *hTestChatRepo {
	return &hTestChatRepo{
		chats:      map[string]*models.Chat{},
		members:    map[string][]string{},
		memberRole: map[string]map[string]models.ChatMemberRole{},
	}
}

func (m *hTestChatRepo) Create(chat *models.Chat) error {
	if chat.ID == "" {
		chat.ID = "chat-1"
	}
	m.chats[chat.ID] = chat
	return nil
}

func (m *hTestChatRepo) FindByID(id string) (*models.Chat, error) {
	if c, ok := m.chats[id]; ok {
		return c, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestChatRepo) FindByUserID(userID string) ([]*models.Chat, error) {
	var out []*models.Chat
	for id, ids := range m.members {
		for _, uid := range ids {
			if uid == userID {
				out = append(out, m.chats[id])
				break
			}
		}
	}
	return out, nil
}

func (m *hTestChatRepo) FindSavedChat(userID string) (*models.Chat, error) {
	for id, ids := range m.members {
		if len(ids) == 1 && ids[0] == userID && m.chats[id].Type == models.ChatTypePersonal {
			return m.chats[id], nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *hTestChatRepo) AddMember(chatID, userID string, role models.ChatMemberRole) error {
	m.members[chatID] = append(m.members[chatID], userID)
	if m.memberRole[chatID] == nil {
		m.memberRole[chatID] = map[string]models.ChatMemberRole{}
	}
	m.memberRole[chatID][userID] = role
	return nil
}

func (m *hTestChatRepo) RemoveMember(chatID, userID string) error {
	var keep []string
	for _, uid := range m.members[chatID] {
		if uid != userID {
			keep = append(keep, uid)
		}
	}
	m.members[chatID] = keep
	return nil
}

func (m *hTestChatRepo) GetMembers(chatID string) ([]*models.ChatMember, error) {
	var out []*models.ChatMember
	for _, uid := range m.members[chatID] {
		out = append(out, &models.ChatMember{ChatID: chatID, UserID: uid, Role: m.memberRole[chatID][uid]})
	}
	return out, nil
}

func (m *hTestChatRepo) GetMemberIDs(chatID string) ([]string, error) {
	return m.members[chatID], nil
}

func (m *hTestChatRepo) IsMember(chatID, userID string) (bool, error) {
	for _, uid := range m.members[chatID] {
		if uid == userID {
			return true, nil
		}
	}
	return false, nil
}

// hTestMsgRepo — in-memory реализация service.MessageRepository для handler-тестов.
type hTestMsgRepo struct {
	messages map[string]*models.Message
}

func newHTestMsgRepo() *hTestMsgRepo {
	return &hTestMsgRepo{messages: map[string]*models.Message{}}
}

func (m *hTestMsgRepo) Create(msg *models.Message) error {
	if msg.ID == "" {
		msg.ID = "msg-1"
	}
	m.messages[msg.ID] = msg
	return nil
}

func (m *hTestMsgRepo) FindByID(id string) (*models.Message, error) {
	if msg, ok := m.messages[id]; ok {
		return msg, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestMsgRepo) FindByChatID(chatID string, limit, offset int) ([]*models.Message, error) {
	var out []*models.Message
	for _, msg := range m.messages {
		if msg.ChatID == chatID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *hTestMsgRepo) FindByChatIDAfterSeq(chatID string, afterSeq int64, limit int) ([]*models.Message, error) {
	var out []*models.Message
	for _, msg := range m.messages {
		if msg.ChatID == chatID && msg.Seq > afterSeq {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *hTestMsgRepo) GetLastSeq(chatID string) (int64, error) {
	var max int64
	for _, msg := range m.messages {
		if msg.ChatID == chatID && msg.Seq > max {
			max = msg.Seq
		}
	}
	return max, nil
}

func (m *hTestMsgRepo) Update(msg *models.Message) error {
	m.messages[msg.ID] = msg
	return nil
}

func (m *hTestMsgRepo) Delete(id string) error {
	delete(m.messages, id)
	return nil
}

func (m *hTestMsgRepo) UpdateStatus(id string, status models.MessageStatus) error { return nil }

func (m *hTestMsgRepo) UpdateLastRead(chatID, userID, upToMessageID string) error { return nil }

func (m *hTestMsgRepo) UpsertLinkPreview(msgID string, p *models.LinkPreview) error { return nil }

func (m *hTestMsgRepo) FindLinkPreviewsByMessageIDs(msgIDs []string) (map[string]*models.LinkPreview, error) {
	return nil, nil
}

func (m *hTestMsgRepo) MarkRead(chatID, readerID, upToMessageID string) error { return nil }

// hTestHub — mock wsHub для handler-тестов.
type hTestHub struct {
	broadcasts int
}

func (h *hTestHub) BroadcastToChat(userIDs []string, data []byte) { h.broadcasts++ }
func (h *hTestHub) IsOnline(userID string) bool                   { return false }
func (h *hTestHub) SendToUser(userID string, data []byte)        {}

func withClaims(req *http.Request, userID string) *http.Request {
	claims := &auth.Claims{UserID: userID, Username: "tester", Role: "user"}
	return req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, claims))
}

func newChatRouter(t *testing.T, chatRepo service.ChatRepository, msgRepo service.MessageRepository) http.Handler {
	t.Helper()
	svc := service.NewChatService(chatRepo, msgRepo, &hTestHub{})
	h := NewChatHandler(svc)
	r := chi.NewRouter()
	r.Route("/chats", func(r chi.Router) {
		r.Post("/", h.CreateChat)
		r.Get("/", h.ListChats)
		r.Post("/{id}/messages", h.SendMessage)
		r.Get("/{id}/messages", h.GetMessages)
		r.Post("/{id}/read", h.MarkRead)
		r.Post("/{id}/typing", h.Typing)
		r.Post("/{id}/members", h.AddMember)
		r.Delete("/{id}/members/{userId}", h.RemoveMember)
		r.Get("/{id}", h.GetChat)
		r.Get("/{id}/members", h.GetMembers)
		r.Patch("/messages/{msgId}", h.EditMessage)
		r.Delete("/messages/{msgId}", h.DeleteMessage)
	})
	return r
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v, body=%s", err, w.Body.String())
	}
	return resp
}

func TestChatHandlerCreateChat(t *testing.T) {
	chatRepo := newHTestChatRepo()
	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	body := `{"type":"group","title":"Dev Team","members":["bob"]}`
	req := httptest.NewRequest(http.MethodPost, "/chats/", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w)
	if resp["error"] != false {
		t.Errorf("expected error=false, got %v", resp["error"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["title"] != "Dev Team" {
		t.Errorf("expected title Dev Team, got %v", data["title"])
	}
	if !chatRepo.IsMemberBool("chat-1", "alice") || !chatRepo.IsMemberBool("chat-1", "bob") {
		t.Error("expected creator and members to be added")
	}
}

func (m *hTestChatRepo) IsMemberBool(chatID, userID string) bool {
	ok, _ := m.IsMember(chatID, userID)
	return ok
}

func TestChatHandlerCreateChatTooLongTitle(t *testing.T) {
	router := newChatRouter(t, newHTestChatRepo(), newHTestMsgRepo())

	long := `"` + string(bytes.Repeat([]byte("x"), 256)) + `"`
	body := `{"type":"group","title":` + long + `}`
	req := httptest.NewRequest(http.MethodPost, "/chats/", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandlerCreateChatInvalidBody(t *testing.T) {
	router := newChatRouter(t, newHTestChatRepo(), newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodPost, "/chats/", bytes.NewBufferString("{not json"))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandlerGetChatNotMember(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chat := &models.Chat{ID: "chat-9", Type: models.ChatTypeGroup, Title: "Secret"}
	chatRepo.chats["chat-9"] = chat
	chatRepo.AddMember("chat-9", "bob", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodGet, "/chats/chat-9", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member, got %d", w.Code)
	}
}

func TestChatHandlerGetChatNotFound(t *testing.T) {
	chatRepo := newHTestChatRepo()
	// членство есть, но самого чата в репо нет -> FindByID вернёт ErrNoRows
	chatRepo.AddMember("ghost", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodGet, "/chats/ghost", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestChatHandlerGetChatOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chat := &models.Chat{ID: "chat-9", Type: models.ChatTypeGroup, Title: "Team"}
	chatRepo.chats["chat-9"] = chat
	chatRepo.AddMember("chat-9", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodGet, "/chats/chat-9", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestChatHandlerSendMessageOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleMember)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	body := `{"content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/messages", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["content"] != "hello" {
		t.Errorf("expected content hello, got %v", data["content"])
	}
}

func TestChatHandlerSendMessageNotMember(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	body := `{"content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/messages", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChatHandlerSendMessageEmptyContent(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	body := `{"content":""}`
	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/messages", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandlerSendMessageTooLong(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	long := string(bytes.Repeat([]byte("x"), 4097))
	body := `{"content":"` + long + `"}`
	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/messages", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandlerGetMessagesOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	msgRepo := newHTestMsgRepo()
	msgRepo.Create(&models.Message{ID: "m1", ChatID: "chat-1", SenderID: "alice", Content: "hi"})

	router := newChatRouter(t, chatRepo, msgRepo)

	req := httptest.NewRequest(http.MethodGet, "/chats/chat-1/messages?limit=10", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %v", resp["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 message, got %d", len(data))
	}
}

func TestChatHandlerGetMessagesNotMember(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodGet, "/chats/chat-1/messages", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChatHandlerTypingNotMember(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/typing", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChatHandlerTypingOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/typing", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestChatHandlerMarkReadNotMember(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	body := `{"message_id":"m1"}`
	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/read", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChatHandlerMarkReadMissingMessageID(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodPost, "/chats/chat-1/read", bytes.NewBufferString(`{}`))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestChatHandlerEditMessageNoPermission(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	msgRepo := newHTestMsgRepo()
	msgRepo.Create(&models.Message{ID: "m1", ChatID: "chat-1", SenderID: "bob", Content: "orig"})

	router := newChatRouter(t, chatRepo, msgRepo)

	req := httptest.NewRequest(http.MethodPatch, "/chats/messages/m1", bytes.NewBufferString(`{"content":"new"}`))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChatHandlerEditMessageOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	msgRepo := newHTestMsgRepo()
	msgRepo.Create(&models.Message{ID: "m1", ChatID: "chat-1", SenderID: "alice", Content: "orig"})

	router := newChatRouter(t, chatRepo, msgRepo)

	req := httptest.NewRequest(http.MethodPatch, "/chats/messages/m1", bytes.NewBufferString(`{"content":"edited"}`))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestChatHandlerDeleteMessageNoPermission(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	msgRepo := newHTestMsgRepo()
	msgRepo.Create(&models.Message{ID: "m1", ChatID: "chat-1", SenderID: "bob", Content: "orig"})

	router := newChatRouter(t, chatRepo, msgRepo)

	req := httptest.NewRequest(http.MethodDelete, "/chats/messages/m1", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestChatHandlerDeleteMessageOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	msgRepo := newHTestMsgRepo()
	msgRepo.Create(&models.Message{ID: "m1", ChatID: "chat-1", SenderID: "alice", Content: "orig"})

	router := newChatRouter(t, chatRepo, msgRepo)

	req := httptest.NewRequest(http.MethodDelete, "/chats/messages/m1", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if _, ok := msgRepo.messages["m1"]; ok {
		t.Error("message should be deleted")
	}
}

func TestChatHandlerRemoveMemberNoPermission(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleMember)
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleMember)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodDelete, "/chats/chat-1/members/bob", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for non-owner removal, got %d", w.Code)
	}
}

func TestChatHandlerRemoveMemberOk(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)
	chatRepo.AddMember("chat-1", "bob", models.ChatMemberRoleMember)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodDelete, "/chats/chat-1/members/bob", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if chatRepo.IsMemberBool("chat-1", "bob") {
		t.Error("bob should be removed")
	}
}

func TestChatHandlerEditMessageNotFound(t *testing.T) {
	chatRepo := newHTestChatRepo()
	chatRepo.AddMember("chat-1", "alice", models.ChatMemberRoleOwner)

	router := newChatRouter(t, chatRepo, newHTestMsgRepo())

	req := httptest.NewRequest(http.MethodPatch, "/chats/messages/ghost", bytes.NewBufferString(`{"content":"new"}`))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 (mapped to ErrChatNotFound), got %d", w.Code)
	}
}

var _ = errors.Is
