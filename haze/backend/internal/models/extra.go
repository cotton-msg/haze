package models

import "time"

type Topic struct {
	ID            string     `json:"id" db:"id"`
	ChatID        string     `json:"chat_id" db:"chat_id"`
	Title         string     `json:"title" db:"title"`
	IsPinned      bool       `json:"is_pinned" db:"is_pinned"`
	MessageCount  int        `json:"message_count" db:"message_count"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty" db:"last_message_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

type ChatFolder struct {
	ID       string `json:"id" db:"id"`
	UserID   string `json:"user_id" db:"user_id"`
	Name     string `json:"name" db:"name"`
	Icon     string `json:"icon" db:"icon"`
	Emoji    string `json:"emoji" db:"emoji"`
	Position int    `json:"position" db:"position"`
}
