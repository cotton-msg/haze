package models

import "time"

type User struct {
	ID          string    `json:"id" db:"id"`
	SSAID       string    `json:"ssa_id" db:"ssa_id"`
	Username    string    `json:"username" db:"username"`
	Email       string    `json:"email" db:"email"`
	Phone       string    `json:"phone" db:"phone"`
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Bio         string    `json:"bio" db:"bio"`
	StatusText  string    `json:"status_text" db:"status_text"`
	Role        string    `json:"role" db:"role"`
	IsPremium   bool      `json:"is_premium" db:"is_premium"`
	LastSeenAt  time.Time `json:"last_seen_at" db:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type Session struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	RefreshToken string    `json:"-" db:"refresh_token"`
	UserAgent    string    `json:"user_agent" db:"user_agent"`
	IP           string    `json:"ip" db:"ip"`
	IsRevoked    bool      `json:"is_revoked" db:"is_revoked"`
	ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type UserBadge struct {
	ID         string    `json:"id" db:"id"`
	UserID     string    `json:"user_id" db:"user_id"`
	BadgeType  string    `json:"badge_type" db:"badge_type"`
	AssignedBy string    `json:"assigned_by" db:"assigned_by"`
	AssignedAt time.Time `json:"assigned_at" db:"assigned_at"`
}
