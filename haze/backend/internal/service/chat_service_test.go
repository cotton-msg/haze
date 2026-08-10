package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/cotton-msg/haze/backend/internal/models"
)

// mockChatRepo — in-memory реализация ChatRepository для тестов.
type mockChatRepo struct {
	chats      map[string]*models.Chat
	members    map[string][]string // chatID -> userIDs
	memberRole map[string]map[string]models.ChatMemberRole
	created    *models.Chat
	added      [][]string
	removed    [][]string
}

func newMockChatRepo() *mockChatRepo {
	return &mockChatRepo{
		chats:      map[string]*models.Chat{},
		members:    map[string][]string{},
		memberRole: map[string]map[string]models.ChatMemberRole{},
	}
}

func (m *mockChatRepo) Create(chat *models.Chat) error {
	if chat.ID == "" {
		chat.ID = "chat-1"
	}
	m.created = chat
	m.chats[chat.ID] = chat
	return nil
}

func (m *mockChatRepo) FindByID(id string) (*models.Chat, error) {
	if c, ok := m.chats[id]; ok {
		return c, nil
	}
	return nil, errors.New("sql: no rows")
}

func (m *mockChatRepo) FindByUserID(userID string) ([]*models.Chat, error) {
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

func (m *mockChatRepo) FindSavedChat(userID string) (*models.Chat, error) {
	for id, ids := range m.members {
		if len(ids) == 1 && ids[0] == userID && m.chats[id].Type == models.ChatTypePersonal {
			return m.chats[id], nil
		}
	}
	return nil, errors.New("sql: no rows")
}

func (m *mockChatRepo) AddMember(chatID, userID string, role models.ChatMemberRole) error {
	m.members[chatID] = append(m.members[chatID], userID)
	if m.memberRole[chatID] == nil {
		m.memberRole[chatID] = map[string]models.ChatMemberRole{}
	}
	m.memberRole[chatID][userID] = role
	m.added = append(m.added, []string{chatID, userID})
	return nil
}

func (m *mockChatRepo) RemoveMember(chatID, userID string) error {
	var keep []string
	for _, uid := range m.members[chatID] {
		if uid != userID {
			keep = append(keep, uid)
		}
	}
	m.members[chatID] = keep
	m.removed = append(m.removed, []string{chatID, userID})
	return nil
}

func (m *mockChatRepo) GetMembers(chatID string) ([]*models.ChatMember, error) {
	var out []*models.ChatMember
	for _, uid := range m.members[chatID] {
		role := m.memberRole[chatID][uid]
		out = append(out, &models.ChatMember{ChatID: chatID, UserID: uid, Role: role})
	}
	return out, nil
}

func (m *mockChatRepo) GetMemberIDs(chatID string) ([]string, error) {
	return m.members[chatID], nil
}

func (m *mockChatRepo) IsMember(chatID, userID string) (bool, error) {
	for _, uid := range m.members[chatID] {
		if uid == userID {
			return true, nil
		}
	}
	return false, nil
}

// mockMsgRepo — in-memory реализация MessageRepository для тестов.
type mockMsgRepo struct {
	messages map[string]*models.Message
	created  *models.Message
	updated  *models.Message
	deleted  []string
	markRead [][]string
}

func newMockMsgRepo() *mockMsgRepo {
	return &mockMsgRepo{messages: map[string]*models.Message{}}
}

func (m *mockMsgRepo) Create(msg *models.Message) error {
	if msg.ID == "" {
		msg.ID = "msg-1"
	}
	m.created = msg
	m.messages[msg.ID] = msg
	return nil
}

func (m *mockMsgRepo) FindByID(id string) (*models.Message, error) {
	if msg, ok := m.messages[id]; ok {
		return msg, nil
	}
	return nil, errors.New("sql: no rows")
}

func (m *mockMsgRepo) FindByChatID(chatID string, limit, offset int) ([]*models.Message, error) {
	var out []*models.Message
	for _, msg := range m.messages {
		if msg.ChatID == chatID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *mockMsgRepo) FindByChatIDAfterSeq(chatID string, afterSeq int64, limit int) ([]*models.Message, error) {
	var out []*models.Message
	for _, msg := range m.messages {
		if msg.ChatID == chatID && msg.Seq > afterSeq {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *mockMsgRepo) GetLastSeq(chatID string) (int64, error) {
	var max int64
	for _, msg := range m.messages {
		if msg.ChatID == chatID && msg.Seq > max {
			max = msg.Seq
		}
	}
	return max, nil
}

func (m *mockMsgRepo) Update(msg *models.Message) error {
	m.updated = msg
	m.messages[msg.ID] = msg
	return nil
}

func (m *mockMsgRepo) Delete(id string) error {
	m.deleted = append(m.deleted, id)
	delete(m.messages, id)
	return nil
}

func (m *mockMsgRepo) UpdateStatus(id string, status models.MessageStatus) error {
	if msg, ok := m.messages[id]; ok {
		msg.Status = status
	}
	return nil
}

func (m *mockMsgRepo) UpdateLastRead(chatID, userID, upToMessageID string) error {
	return nil
}

func (m *mockMsgRepo) MarkRead(chatID, readerID, upToMessageID string) error {
	m.markRead = append(m.markRead, []string{chatID, readerID, upToMessageID})
	return nil
}

func (m *mockMsgRepo) UpsertLinkPreview(msgID string, p *models.LinkPreview) error {
	return nil
}

func (m *mockMsgRepo) FindLinkPreviewsByMessageIDs(msgIDs []string) (map[string]*models.LinkPreview, error) {
	return nil, nil
}

// mockHub — собирает broadcast-события для проверки.
type mockHub struct {
	broadcasts [][]byte
	userIDs    []string
	online     map[string]bool
}

func (m *mockHub) BroadcastToChat(userIDs []string, data []byte) {
	m.broadcasts = append(m.broadcasts, data)
	m.userIDs = userIDs
}

func (m *mockHub) IsOnline(userID string) bool { return m.online[userID] }

func (m *mockHub) SendToUser(userID string, data []byte) {
	m.broadcasts = append(m.broadcasts, data)
}

func newTestChatService(chat *mockChatRepo, msg *mockMsgRepo, hub *mockHub) *ChatService {
	return NewChatService(chat, msg, hub)
}

func TestCreateChatAddsCreatorAsOwner(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	chat, err := svc.CreateChat("user-1", CreateChatInput{Type: models.ChatTypeGroup, Title: "Test", Members: []string{"user-2"}})
	if err != nil {
		t.Fatalf("CreateChat: %v", err)
	}
	if chatRepo.memberRole[chat.ID]["user-1"] != models.ChatMemberRoleOwner {
		t.Errorf("creator should be owner, got %s", chatRepo.memberRole[chat.ID]["user-1"])
	}
	if chatRepo.memberRole[chat.ID]["user-2"] != models.ChatMemberRoleMember {
		t.Errorf("member-2 should be member, got %s", chatRepo.memberRole[chat.ID]["user-2"])
	}
}

func TestSendMessageAsNonMemberRejected(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	_, err := svc.SendMessage("intruder", SendMessageInput{ChatID: "chat-1", Content: "hi"})
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestSendMessageBroadcasts(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msg, err := svc.SendMessage("user-1", SendMessageInput{ChatID: "chat-1", Content: "hello"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if msg.Content != "hello" {
		t.Errorf("unexpected content: %s", msg.Content)
	}
	if len(hub.broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(hub.broadcasts))
	}
}

func TestGetMessagesAsNonMemberRejected(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	_, err := svc.GetMessages("chat-1", "intruder", 50, 0)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestGetChatAsNonMemberRejected(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.chats["chat-1"] = &models.Chat{ID: "chat-1", Title: "X"}
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	_, err := svc.GetChat("chat-1", "intruder")
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestEditMessageWrongSenderRejected(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	msgRepo.messages["msg-1"] = &models.Message{ID: "msg-1", ChatID: "chat-1", SenderID: "user-1", Content: "orig"}
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)
	chatRepo.members["chat-1"] = []string{"user-1"}

	_, err := svc.EditMessage("user-2", "msg-1", "hacked")
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("expected ErrNoPermission, got %v", err)
	}
}

func TestDeleteMessageRejectsNonMemberSender(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	// Сообщение принадлежит user-1, но user-1 уже не член чата.
	msgRepo.messages["msg-1"] = &models.Message{ID: "msg-1", ChatID: "chat-1", SenderID: "user-1", Content: "orig"}
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	err := svc.DeleteMessage("user-1", "msg-1")
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestAddMemberNonMemberActorRejected(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	err := svc.AddMember("chat-1", "intruder", "user-3")
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestRemoveMemberRequiresAdminRole(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	chatRepo.memberRole["chat-1"] = map[string]models.ChatMemberRole{
		"user-1": models.ChatMemberRoleMember,
		"user-2": models.ChatMemberRoleMember,
	}
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	err := svc.RemoveMember("chat-1", "user-1", "user-2")
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("expected ErrNoPermission, got %v", err)
	}
}

func TestSendMessageEmitsDeliveredToSender(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msgRepo := newMockMsgRepo()
	hub := &mockHub{online: map[string]bool{"user-2": true}}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	msg, err := svc.SendMessage("user-1", SendMessageInput{ChatID: "chat-1", Content: "hi"})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	// 2 события: new_message + delivered
	if len(hub.broadcasts) != 2 {
		t.Fatalf("expected new_message + delivered, got %d events", len(hub.broadcasts))
	}
	last := hub.broadcasts[len(hub.broadcasts)-1]
	var ev struct {
		Type    string `json:"type"`
		Payload struct {
			ChatID    string `json:"chat_id"`
			MessageID string `json:"message_id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(last, &ev); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if ev.Type != "delivered" {
		t.Errorf("expected delivered event, got %s", ev.Type)
	}
	if ev.Payload.ChatID != "chat-1" || ev.Payload.MessageID != msg.ID {
		t.Errorf("unexpected delivered payload: %+v", ev.Payload)
	}
}

func TestSendMessageNoDeliveredWhenRecipientOffline(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	if _, err := svc.SendMessage("user-1", SendMessageInput{ChatID: "chat-1", Content: "hi"}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if len(hub.broadcasts) != 1 {
		t.Fatalf("expected only new_message, got %d events", len(hub.broadcasts))
	}
}

func TestEditMessageBroadcastsUpdateEvent(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msgRepo := newMockMsgRepo()
	msgRepo.messages["msg-1"] = &models.Message{ID: "msg-1", ChatID: "chat-1", SenderID: "user-1", Content: "old"}
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	msg, err := svc.EditMessage("user-1", "msg-1", "new")
	if err != nil {
		t.Fatalf("EditMessage: %v", err)
	}
	if msg.Content != "new" {
		t.Errorf("expected content 'new', got %s", msg.Content)
	}
	if len(hub.broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(hub.broadcasts))
	}
	var ev struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(hub.broadcasts[0], &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "message_updated" {
		t.Errorf("expected message_updated event, got %s", ev.Type)
	}
}

func TestDeleteMessageBroadcastsDeleteEvent(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msgRepo := newMockMsgRepo()
	msgRepo.messages["msg-1"] = &models.Message{ID: "msg-1", ChatID: "chat-1", SenderID: "user-1", Content: "hi"}
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	if err := svc.DeleteMessage("user-1", "msg-1"); err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}
	if len(hub.broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(hub.broadcasts))
	}
	var ev struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(hub.broadcasts[0], &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "message_deleted" {
		t.Errorf("expected message_deleted event, got %s", ev.Type)
	}
}

func TestMarkReadBroadcastsReadEvent(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	if err := svc.MarkRead("chat-1", "user-2", "msg-9"); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if len(hub.broadcasts) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(hub.broadcasts))
	}
	if len(msgRepo.markRead) != 1 {
		t.Fatalf("expected MarkRead call, got %d", len(msgRepo.markRead))
	}
}

func TestBroadcastTypingRequiresMembership(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	err := svc.BroadcastTyping("chat-1", "intruder")
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}

func TestSyncChatReturnsIncrementalMessages(t *testing.T) {
	chatRepo := newMockChatRepo()
	chatRepo.members["chat-1"] = []string{"user-1", "user-2"}
	msgRepo := newMockMsgRepo()
	msgRepo.messages["msg-1"] = &models.Message{ID: "msg-1", ChatID: "chat-1", Seq: 1, Content: "one"}
	msgRepo.messages["msg-2"] = &models.Message{ID: "msg-2", ChatID: "chat-1", Seq: 2, Content: "two"}
	msgRepo.messages["msg-3"] = &models.Message{ID: "msg-3", ChatID: "chat-1", Seq: 3, Content: "three"}
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	res, err := svc.SyncChat("chat-1", "user-2", 1, 100)
	if err != nil {
		t.Fatalf("SyncChat: %v", err)
	}
	if res.LastSeq != 3 {
		t.Errorf("expected last_seq 3, got %d", res.LastSeq)
	}
	if len(res.Messages) != 2 {
		t.Fatalf("expected 2 messages after seq 1, got %d", len(res.Messages))
	}
	if res.HasMore {
		t.Error("unexpected has_more=true")
	}
	if res.Messages[0].Seq != 2 || res.Messages[1].Seq != 3 {
		t.Errorf("messages not ordered by seq asc: %d, %d", res.Messages[0].Seq, res.Messages[1].Seq)
	}
}

func TestSyncChatNonMemberRejected(t *testing.T) {
	chatRepo := newMockChatRepo()
	msgRepo := newMockMsgRepo()
	hub := &mockHub{}
	svc := newTestChatService(chatRepo, msgRepo, hub)

	_, err := svc.SyncChat("chat-1", "intruder", 0, 100)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("expected ErrNotMember, got %v", err)
	}
}
