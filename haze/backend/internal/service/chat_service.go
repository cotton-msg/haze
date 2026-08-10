package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/pkg/unfurl"
)

func toJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

var (
	ErrChatNotFound = errors.New("chat not found")
	ErrNotMember    = errors.New("user is not a member of this chat")
	ErrNoPermission = errors.New("no permission")
)

type ChatService struct {
	chatRepo  ChatRepository
	msgRepo   MessageRepository
	hub       wsHub
	notifSvc  *NotificationService
	indexer   Indexer
	pushNotif PushNotifier
	cache     MessageCache
	listCache ChatListCache
	presence  *PresenceService
	unfurl    UnfurlProducer
}

// UnfurlProducer ставит задачу разбора OG-метаданных в очередь.
type UnfurlProducer interface {
	EnqueueUnfurl(messageID, url string) error
}

// ChatRepository — интерфейс чатов для сервисного слоя (тестируемость).
type ChatRepository interface {
	Create(chat *models.Chat) error
	FindByID(id string) (*models.Chat, error)
	FindByUserID(userID string) ([]*models.Chat, error)
	FindSavedChat(userID string) (*models.Chat, error)
	AddMember(chatID, userID string, role models.ChatMemberRole) error
	RemoveMember(chatID, userID string) error
	GetMembers(chatID string) ([]*models.ChatMember, error)
	GetMemberIDs(chatID string) ([]string, error)
	IsMember(chatID, userID string) (bool, error)
}

// MessageRepository — интерфейс сообщений для сервисного слоя.
type MessageRepository interface {
	Create(msg *models.Message) error
	FindByID(id string) (*models.Message, error)
	FindByChatID(chatID string, limit, offset int) ([]*models.Message, error)
	FindByChatIDAfterSeq(chatID string, afterSeq int64, limit int) ([]*models.Message, error)
	GetLastSeq(chatID string) (int64, error)
	Update(msg *models.Message) error
	Delete(id string) error
	UpdateStatus(id string, status models.MessageStatus) error
	UpdateLastRead(chatID, userID, upToMessageID string) error
	MarkRead(chatID, readerID, upToMessageID string) error
	UpsertLinkPreview(msgID string, p *models.LinkPreview) error
	FindLinkPreviewsByMessageIDs(msgIDs []string) (map[string]*models.LinkPreview, error)
}

type wsHub interface {
	BroadcastToChat(userIDs []string, data []byte)
	IsOnline(userID string) bool
	SendToUser(userID string, data []byte)
}

func NewChatService(
	chatRepo ChatRepository,
	msgRepo MessageRepository,
	hub wsHub,
) *ChatService {
	return &ChatService{
		chatRepo:  chatRepo,
		msgRepo:   msgRepo,
		hub:       hub,
		cache:     NoopMessageCache{},
		listCache: NoopChatListCache{},
	}
}

// SetMessageCache подключает кэш последних сообщений (Redis).
func (s *ChatService) SetMessageCache(cache MessageCache) {
	if cache != nil {
		s.cache = cache
	}
}

// SetChatListCache подключает кэш списка чатов пользователя (Redis).
func (s *ChatService) SetChatListCache(cache ChatListCache) {
	if cache != nil {
		s.listCache = cache
	}
}

// SetPresenceService подключает отслеживание онлайн-статусов.
func (s *ChatService) SetPresenceService(p *PresenceService) {
	s.presence = p
}

// GetPresence возвращает онлайн-статус пользователя.
func (s *ChatService) GetPresence(userID string) Presence {
	if s.presence != nil {
		return s.presence.Get(userID)
	}
	return Presence{}
}

func (s *ChatService) SetIndexer(i Indexer) {
	s.indexer = i
}

func (s *ChatService) SetPushNotifier(n PushNotifier) {
	s.pushNotif = n
}

// SetUnfurlProducer подключает очередь разбора ссылок (unfurl).
func (s *ChatService) SetUnfurlProducer(p UnfurlProducer) {
	s.unfurl = p
}

type CreateChatInput struct {
	Type    models.ChatType `json:"type"`
	Title   string          `json:"title"`
	Members []string        `json:"members"`
}

func (s *ChatService) CreateChat(creatorID string, input CreateChatInput) (*models.Chat, error) {
	chat := &models.Chat{
		Type:     input.Type,
		Title:    input.Title,
		IsPinned: false,
	}

	if err := s.chatRepo.Create(chat); err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	allMembers := append([]string{creatorID}, input.Members...)
	for _, userID := range allMembers {
		role := models.ChatMemberRoleMember
		if userID == creatorID {
			role = models.ChatMemberRoleOwner
		}
		if err := s.chatRepo.AddMember(chat.ID, userID, role); err != nil {
			return nil, fmt.Errorf("failed to add member: %w", err)
		}
		s.listCache.InvalidateUser(userID)
	}

	return chat, nil
}

func (s *ChatService) GetUserChats(userID string) ([]*models.Chat, error) {
	if cached, ok := s.listCache.GetUserChats(userID); ok {
		return cached, nil
	}
	chats, err := s.chatRepo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}
	s.listCache.SetUserChats(userID, chats)
	return chats, nil
}

// SavedChat возвращает «Избранное» — личный чат пользователя с собой,
// создавая его при первом обращении.
func (s *ChatService) SavedChat(userID string) (*models.Chat, error) {
	chat, err := s.chatRepo.FindSavedChat(userID)
	if err == nil {
		return chat, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to find saved chat: %w", err)
	}

	chat = &models.Chat{Type: models.ChatTypePersonal, Title: "Избранное"}
	if err := s.chatRepo.Create(chat); err != nil {
		return nil, fmt.Errorf("failed to create saved chat: %w", err)
	}
	if err := s.chatRepo.AddMember(chat.ID, userID, models.ChatMemberRoleOwner); err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}
	s.listCache.InvalidateUser(userID)
	return chat, nil
}

func (s *ChatService) GetChat(id, userID string) (*models.Chat, error) {
	isMember, err := s.chatRepo.IsMember(id, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotMember
	}

	chat, err := s.chatRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrChatNotFound
		}
		return nil, fmt.Errorf("failed to find chat: %w", err)
	}
	return chat, nil
}

func (s *ChatService) GetMembers(chatID, userID string) ([]*models.ChatMember, error) {
	isMember, err := s.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotMember
	}
	return s.chatRepo.GetMembers(chatID)
}

type SendMessageInput struct {
	ChatID   string             `json:"chat_id"`
	Content  string             `json:"content"`
	Type     models.MessageType `json:"type"`
	ReplyTo  string             `json:"reply_to,omitempty"`
	ClientID string             `json:"client_id,omitempty"`
}

// ChatSyncResult — результат синхронизации чата для устройства.
type ChatSyncResult struct {
	Messages []*models.Message `json:"messages"`
	LastSeq  int64             `json:"last_seq"`
	HasMore  bool              `json:"has_more"`
}

// SyncChat возвращает все сообщения чата с seq > afterSeq — якорь
// multi-device синхронизации. Клиент повторяет запрос с новым after_seq.
func (s *ChatService) SyncChat(chatID, userID string, afterSeq int64, limit int) (*ChatSyncResult, error) {
	isMember, err := s.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotMember
	}

	lastSeq, err := s.msgRepo.GetLastSeq(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get last seq: %w", err)
	}

	messages, err := s.msgRepo.FindByChatIDAfterSeq(chatID, afterSeq, limit+1)
	if err != nil {
		return nil, fmt.Errorf("failed to sync chat: %w", err)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	if messages == nil {
		messages = make([]*models.Message, 0)
	}

	return &ChatSyncResult{Messages: messages, LastSeq: lastSeq, HasMore: hasMore}, nil
}

func (s *ChatService) SendMessage(senderID string, input SendMessageInput) (*models.Message, error) {
	isMember, err := s.chatRepo.IsMember(input.ChatID, senderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotMember
	}

	msg := &models.Message{
		ChatID:   input.ChatID,
		SenderID: senderID,
		ClientID: input.ClientID,
		Content:  input.Content,
		Type:     input.Type,
		ReplyTo:  input.ReplyTo,
		Status:   models.MessageStatusSent,
	}

	if err := s.msgRepo.Create(msg); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	s.cache.PushRecent(input.ChatID, msg)

	memberIDs, err := s.chatRepo.GetMemberIDs(input.ChatID)
	if err == nil {
		// Новое сообщение меняет список чатов и счётчики непрочитанных участников.
		for _, uid := range memberIDs {
			s.listCache.InvalidateUser(uid)
		}
		event := map[string]interface{}{
			"type": "new_message",
			"payload": map[string]interface{}{
				"message": msg,
			},
		}
		if data, err := toJSON(event); err == nil {
			s.hub.BroadcastToChat(memberIDs, data)
		}
		// Доставка: если хотя бы один получатель онлайн — сообщение доставлено.
		delivered := false
		for _, uid := range memberIDs {
			if uid != senderID && s.hub.IsOnline(uid) {
				delivered = true
				break
			}
		}
		if delivered {
			ev := map[string]interface{}{
				"type": "delivered",
				"payload": map[string]interface{}{
					"chat_id":    input.ChatID,
					"message_id": msg.ID,
				},
			}
			if data, err := toJSON(ev); err == nil {
				s.hub.SendToUser(senderID, data)
			}
		}
		if s.indexer != nil {
			go s.indexer.IndexMessage(msg)
		}
		if s.pushNotif != nil {
			recipients := make([]string, 0, len(memberIDs))
			for _, uid := range memberIDs {
				if uid != senderID {
					recipients = append(recipients, uid)
				}
			}
			go s.pushNotif.NotifyMessage(msg, recipients)
		}
		if s.unfurl != nil && msg.Type == models.MessageTypeText {
			for _, u := range unfurl.ExtractURLs(msg.Content) {
				url := u
				go s.unfurl.EnqueueUnfurl(msg.ID, url)
			}
		}
	}

	return msg, nil
}

func (s *ChatService) GetMessages(chatID, userID string, limit, offset int) ([]*models.Message, error) {
	isMember, err := s.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotMember
	}

	// Первая страница истории — из кэша, если он свежий.
	if offset == 0 && limit <= messageCacheCapacity {
		if cached, ok := s.cache.GetRecent(chatID); ok {
			if len(cached) >= limit {
				return cached[:limit], nil
			}
			return cached, nil
		}
	}

	messages, err := s.msgRepo.FindByChatID(chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	s.attachLinkPreviews(messages)

	if offset == 0 {
		s.cache.SetRecent(chatID, messages)
	}

	return messages, nil
}

// attachLinkPreviews подгружает OG-карточки для списка сообщений.
func (s *ChatService) attachLinkPreviews(messages []*models.Message) {
	ids := make([]string, 0, len(messages))
	for _, m := range messages {
		ids = append(ids, m.ID)
	}
	previews, err := s.msgRepo.FindLinkPreviewsByMessageIDs(ids)
	if err != nil {
		return
	}
	for _, m := range messages {
		if p, ok := previews[m.ID]; ok {
			m.LinkPreview = p
		}
	}
}

// UnfurlJob обрабатывает задачу разбора ссылки: тянет OG-метаданные, сохраняет
// превью и рассылает обновлённое сообщение участникам чата.
func (s *ChatService) UnfurlJob(messageID, url string) error {
	msg, err := s.msgRepo.FindByID(messageID)
	if err != nil {
		return fmt.Errorf("unfurl: message not found: %w", err)
	}

	p, err := unfurl.Fetch(context.Background(), url)
	if err != nil {
		return fmt.Errorf("unfurl fetch %s: %w", url, err)
	}

	lp := &models.LinkPreview{
		URL:         p.URL,
		Title:       p.Title,
		Description: p.Description,
		Image:       p.Image,
	}

	if err := s.msgRepo.UpsertLinkPreview(msg.ID, lp); err != nil {
		return fmt.Errorf("unfurl save: %w", err)
	}

	s.cache.Invalidate(msg.ChatID)
	msg.LinkPreview = lp

	memberIDs, err := s.chatRepo.GetMemberIDs(msg.ChatID)
	if err != nil {
		return nil
	}
	event := map[string]interface{}{
		"type": "message_updated",
		"payload": map[string]interface{}{
			"message": msg,
		},
	}
	if data, err := toJSON(event); err == nil {
		s.hub.BroadcastToChat(memberIDs, data)
	}
	return nil
}

// MarkRead помечает сообщения чата прочитанными и уведомляет отправителей
// WS-событием "read".
func (s *ChatService) MarkRead(chatID, readerID, upToMessageID string) error {
	isMember, err := s.chatRepo.IsMember(chatID, readerID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return ErrNotMember
	}

	if err := s.msgRepo.MarkRead(chatID, readerID, upToMessageID); err != nil {
		return fmt.Errorf("failed to mark read: %w", err)
	}
	if err := s.msgRepo.UpdateLastRead(chatID, readerID, upToMessageID); err != nil {
		return fmt.Errorf("failed to update last read: %w", err)
	}

	// Прочтение меняет счётчик непрочитанных читателя.
	s.listCache.InvalidateUser(readerID)

	memberIDs, err := s.chatRepo.GetMemberIDs(chatID)
	if err == nil {
		event := map[string]interface{}{
			"type": "read",
			"payload": map[string]interface{}{
				"chat_id":    chatID,
				"reader_id":  readerID,
				"message_id": upToMessageID,
			},
		}
		if data, err := toJSON(event); err == nil {
			s.hub.BroadcastToChat(memberIDs, data)
		}
	}

	return nil
}

// BroadcastTyping рассылает typing-событие остальным участникам чата.
func (s *ChatService) BroadcastTyping(chatID, senderID string) error {
	isMember, err := s.chatRepo.IsMember(chatID, senderID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return ErrNotMember
	}

	memberIDs, err := s.chatRepo.GetMemberIDs(chatID)
	if err != nil {
		return err
	}
	event := map[string]interface{}{
		"type": "typing",
		"payload": map[string]interface{}{
			"chat_id":   chatID,
			"user_id":   senderID,
			"is_typing": true,
		},
	}
	if data, err := toJSON(event); err == nil {
		recipients := make([]string, 0, len(memberIDs))
		for _, uid := range memberIDs {
			if uid != senderID {
				recipients = append(recipients, uid)
			}
		}
		s.hub.BroadcastToChat(recipients, data)
	}
	return nil
}

func (s *ChatService) EditMessage(userID, msgID, content string) (*models.Message, error) {
	msg, err := s.msgRepo.FindByID(msgID)
	if err != nil {
		return nil, ErrChatNotFound
	}
	if msg.SenderID != userID {
		return nil, ErrNoPermission
	}

	isMember, err := s.chatRepo.IsMember(msg.ChatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotMember
	}

	msg.Content = content
	if err := s.msgRepo.Update(msg); err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	s.cache.Invalidate(msg.ChatID)

	if memberIDs, err := s.chatRepo.GetMemberIDs(msg.ChatID); err == nil {
		event := map[string]interface{}{
			"type": "message_updated",
			"payload": map[string]interface{}{
				"message": msg,
			},
		}
		if data, err := toJSON(event); err == nil {
			s.hub.BroadcastToChat(memberIDs, data)
		}
	}

	return msg, nil
}

func (s *ChatService) DeleteMessage(userID, msgID string) error {
	msg, err := s.msgRepo.FindByID(msgID)
	if err != nil {
		return ErrChatNotFound
	}
	if msg.SenderID != userID {
		return ErrNoPermission
	}

	isMember, err := s.chatRepo.IsMember(msg.ChatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return ErrNotMember
	}

	if err := s.msgRepo.Delete(msgID); err != nil {
		return err
	}
	s.cache.Invalidate(msg.ChatID)

	if memberIDs, err := s.chatRepo.GetMemberIDs(msg.ChatID); err == nil {
		event := map[string]interface{}{
			"type": "message_deleted",
			"payload": map[string]interface{}{
				"chat_id":    msg.ChatID,
				"message_id": msgID,
			},
		}
		if data, err := toJSON(event); err == nil {
			s.hub.BroadcastToChat(memberIDs, data)
		}
	}

	return nil
}

func (s *ChatService) AddMember(chatID, actorID, targetID string) error {
	isMember, err := s.chatRepo.IsMember(chatID, actorID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}

	s.listCache.InvalidateUser(actorID)
	s.listCache.InvalidateUser(targetID)
	return s.chatRepo.AddMember(chatID, targetID, models.ChatMemberRoleMember)
}

func (s *ChatService) RemoveMember(chatID, actorID, targetID string) error {
	members, err := s.chatRepo.GetMembers(chatID)
	if err != nil {
		return err
	}

	var actorRole models.ChatMemberRole
	for _, m := range members {
		if m.UserID == actorID {
			actorRole = m.Role
			break
		}
	}

	if actorRole != models.ChatMemberRoleOwner && actorRole != models.ChatMemberRoleAdmin {
		return ErrNoPermission
	}

	if err := s.chatRepo.RemoveMember(chatID, targetID); err != nil {
		return err
	}
	s.listCache.InvalidateUser(actorID)
	s.listCache.InvalidateUser(targetID)
	return nil
}

type NotificationService struct{}

func (n *NotificationService) Notify(userID, event string, data interface{}) {}
