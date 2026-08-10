package models

import "time"

type File struct {
	ID        string    `json:"id" db:"id"`
	MessageID string    `json:"message_id" db:"message_id"`
	URL       string    `json:"url" db:"url"`
	MimeType  string    `json:"mime_type" db:"mime_type"`
	Size      int64     `json:"size" db:"size"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type StickerPack struct {
	ID           string `json:"id" db:"id"`
	Name         string `json:"name" db:"name"`
	IsPremium    bool   `json:"is_premium" db:"is_premium"`
	ThumbnailURL string `json:"thumbnail_url" db:"thumbnail_url"`
}

type Sticker struct {
	ID       string `json:"id" db:"id"`
	Name     string `json:"name" db:"name"`
	ImageURL string `json:"image_url" db:"image_url"`
	PackID   string `json:"pack_id" db:"pack_id"`
}

type Reaction struct {
	ID        string    `json:"id" db:"id"`
	MessageID string    `json:"message_id" db:"message_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Emoji     string    `json:"emoji" db:"emoji"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ReactionCount — агрегированное количество реакций по эмодзи.
type ReactionCount struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}
