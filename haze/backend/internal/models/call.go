package models

import "time"

type CallType string

const (
	CallTypeAudio CallType = "audio"
	CallTypeVideo CallType = "video"
)

type CallStatus string

const (
	CallStatusRinging  CallStatus = "ringing"
	CallStatusActive   CallStatus = "active"
	CallStatusEnded    CallStatus = "ended"
	CallStatusMissed   CallStatus = "missed"
	CallStatusRejected CallStatus = "rejected"
)

type Call struct {
	ID        string     `json:"id" db:"id"`
	CallerID  string     `json:"caller_id" db:"caller_id"`
	CalleeID  string     `json:"callee_id" db:"callee_id"`
	Type      CallType   `json:"type" db:"type"`
	Status    CallStatus `json:"status" db:"status"`
	StartedAt time.Time  `json:"started_at" db:"started_at"`
	EndedAt   time.Time  `json:"ended_at" db:"ended_at"`
}

type CallParticipant struct {
	CallID   string    `json:"call_id" db:"call_id"`
	UserID   string    `json:"user_id" db:"user_id"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}
