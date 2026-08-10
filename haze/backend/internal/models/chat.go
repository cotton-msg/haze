package models

import "time"

type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeFile  MessageType = "file"
	MessageTypeVoice MessageType = "voice"
	MessageTypeVideo MessageType = "video"
)

type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
)

// LinkPreview — OG-карточка для сообщения со ссылкой (unfurl).
type LinkPreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

type Message struct {
	ID        string        `json:"id" db:"id"`
	ChatID    string        `json:"chat_id" db:"chat_id"`
	SenderID  string        `json:"sender_id" db:"sender_id"`
	Seq       int64         `json:"seq" db:"seq"`
	ClientID  string        `json:"client_id,omitempty" db:"client_id"`
	Content   string        `json:"content" db:"content"`
	Type      MessageType   `json:"type" db:"type"`
	ReplyTo   string        `json:"reply_to,omitempty" db:"reply_to"`
	Status    MessageStatus `json:"status" db:"status"`
	EditedAt  time.Time     `json:"edited_at,omitempty" db:"edited_at"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
	// LinkPreview заполняется при выдаче (WS/история), в БД — отдельная таблица.
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
}

type ChatType string

const (
	ChatTypePersonal ChatType = "personal"
	ChatTypeGroup    ChatType = "group"
	ChatTypeChannel  ChatType = "channel"
)

type Chat struct {
	ID              string    `json:"id" db:"id"`
	Type            ChatType  `json:"type" db:"type"`
	Title           string    `json:"title" db:"title"`
	Avatar          string    `json:"avatar" db:"avatar"`
	Description     string    `json:"description" db:"description"`
	IsPinned        bool      `json:"is_pinned" db:"is_pinned"`
	PinnedMessageID string    `json:"pinned_message_id,omitempty" db:"pinned_message_id"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	LastMessage     string    `json:"last_message,omitempty" db:"last_message"`
	UnreadCount     int       `json:"unread_count,omitempty" db:"unread_count"`
}

type ChatMemberRole string

const (
	ChatMemberRoleOwner  ChatMemberRole = "owner"
	ChatMemberRoleAdmin  ChatMemberRole = "admin"
	ChatMemberRoleMember ChatMemberRole = "member"
)

type ChatMember struct {
	ChatID   string         `json:"chat_id" db:"chat_id"`
	UserID   string         `json:"user_id" db:"user_id"`
	Role     ChatMemberRole `json:"role" db:"role"`
	JoinedAt time.Time      `json:"joined_at" db:"joined_at"`
}

type Contact struct {
	UserID    string    `json:"user_id" db:"user_id"`
	ContactID string    `json:"contact_id" db:"contact_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
