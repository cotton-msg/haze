package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/redis/go-redis/v9"
)

// messageCacheTTL — время жизни кэша последних сообщений чата.
const messageCacheTTL = 5 * time.Minute

// messageCacheCapacity — сколько последних сообщений держим в кэше.
const messageCacheCapacity = 100

// MessageCache — кэш последних сообщений чата (ускоряет открытие чата).
type MessageCache interface {
	GetRecent(chatID string) ([]*models.Message, bool)
	SetRecent(chatID string, messages []*models.Message)
	PushRecent(chatID string, msg *models.Message)
	Invalidate(chatID string)
}

// RedisMessageCache — реализация на Redis: JSON-список по ключу.
type RedisMessageCache struct {
	client redis.Cmdable
	prefix string
}

func NewRedisMessageCache(client redis.Cmdable, prefix string) *RedisMessageCache {
	return &RedisMessageCache{client: client, prefix: prefix}
}

func (c *RedisMessageCache) key(chatID string) string {
	return c.prefix + ":msgs:" + chatID
}

func (c *RedisMessageCache) GetRecent(chatID string) ([]*models.Message, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	raw, err := c.client.Get(ctx, c.key(chatID)).Bytes()
	if err != nil {
		return nil, false
	}
	var messages []*models.Message
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, false
	}
	return messages, len(messages) > 0
}

func (c *RedisMessageCache) SetRecent(chatID string, messages []*models.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	data, err := json.Marshal(messages)
	if err != nil {
		return
	}
	c.client.Set(ctx, c.key(chatID), data, messageCacheTTL)
}

// PushRecent добавляет новое сообщение в начало кэша (порядок: от новых к старым).
func (c *RedisMessageCache) PushRecent(chatID string, msg *models.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	messages, ok := c.GetRecent(chatID)
	if !ok {
		return
	}
	messages = append([]*models.Message{msg}, messages...)
	if len(messages) > messageCacheCapacity {
		messages = messages[:messageCacheCapacity]
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return
	}
	c.client.Set(ctx, c.key(chatID), data, messageCacheTTL)
}

func (c *RedisMessageCache) Invalidate(chatID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	c.client.Del(ctx, c.key(chatID))
}

// chatListCacheTTL — короткий TTL: список чатов должен быть почти свежим.
const chatListCacheTTL = 30 * time.Second

// ChatListCache — кэш списка чатов пользователя (включая счётчики непрочитанных).
type ChatListCache interface {
	GetUserChats(userID string) ([]*models.Chat, bool)
	SetUserChats(userID string, chats []*models.Chat)
	InvalidateUser(userID string)
}

// RedisChatListCache — реализация на Redis: JSON-список по ключу пользователя.
type RedisChatListCache struct {
	client redis.Cmdable
	prefix string
}

func NewRedisChatListCache(client redis.Cmdable, prefix string) *RedisChatListCache {
	return &RedisChatListCache{client: client, prefix: prefix}
}

func (c *RedisChatListCache) key(userID string) string {
	return c.prefix + ":chats:" + userID
}

func (c *RedisChatListCache) GetUserChats(userID string) ([]*models.Chat, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	raw, err := c.client.Get(ctx, c.key(userID)).Bytes()
	if err != nil {
		return nil, false
	}
	var chats []*models.Chat
	if err := json.Unmarshal(raw, &chats); err != nil {
		return nil, false
	}
	return chats, true
}

func (c *RedisChatListCache) SetUserChats(userID string, chats []*models.Chat) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	data, err := json.Marshal(chats)
	if err != nil {
		return
	}
	c.client.Set(ctx, c.key(userID), data, chatListCacheTTL)
}

func (c *RedisChatListCache) InvalidateUser(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	c.client.Del(ctx, c.key(userID))
}

// NoopChatListCache — заглушка, когда Redis недоступен.
type NoopChatListCache struct{}

func (NoopChatListCache) GetUserChats(userID string) ([]*models.Chat, bool) { return nil, false }
func (NoopChatListCache) SetUserChats(userID string, chats []*models.Chat)   {}
func (NoopChatListCache) InvalidateUser(userID string)                       {}

// NoopMessageCache — заглушка, когда Redis недоступен.
type NoopMessageCache struct{}

func (NoopMessageCache) GetRecent(chatID string) ([]*models.Message, bool)   { return nil, false }
func (NoopMessageCache) SetRecent(chatID string, messages []*models.Message) {}
func (NoopMessageCache) PushRecent(chatID string, msg *models.Message)       {}
func (NoopMessageCache) Invalidate(chatID string)                            {}
