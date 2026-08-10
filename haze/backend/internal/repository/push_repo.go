package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/cotton-msg/haze/backend/pkg/crypto"
	"github.com/google/uuid"
)

type PushSubscription struct {
	ID        string
	UserID    string
	Endpoint  string
	P256DH    string
	Auth      string
	CreatedAt time.Time
}

type PushRepository struct {
	db  *sql.DB
	box *crypto.Box
}

func NewPushRepository(db *sql.DB, box *crypto.Box) *PushRepository {
	return &PushRepository{db: db, box: box}
}

// encrypt защищает чувствительные поля подписки (p256dh, auth_secret).
// Если шифрование не настроено, значения сохраняются как есть.
func (r *PushRepository) encrypt(values ...string) []string {
	if r.box == nil {
		return values
	}
	out := make([]string, len(values))
	for i, v := range values {
		enc, err := r.box.Encrypt(v)
		if err == nil {
			out[i] = enc
		} else {
			out[i] = v
		}
	}
	return out
}

func (r *PushRepository) decrypt(values ...string) []string {
	if r.box == nil {
		return values
	}
	out := make([]string, len(values))
	for i, v := range values {
		dec, err := r.box.Decrypt(v)
		if err == nil {
			out[i] = dec
		} else {
			out[i] = v
		}
	}
	return out
}

func (r *PushRepository) Save(userID, endpoint, p256dh, auth string) error {
	enc := r.encrypt(p256dh, auth)
	p, a := enc[0], enc[1]
	_, err := r.db.Exec(`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth_secret, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh = $4, auth_secret = $5`,
		uuid.New().String(), userID, endpoint, p, a, time.Now())
	return err
}

func (r *PushRepository) Delete(userID, endpoint string) error {
	_, err := r.db.Exec(`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
	return err
}

func (r *PushRepository) FindByUserID(userID string) ([]*PushSubscription, error) {
	rows, err := r.db.Query(`SELECT id, user_id, endpoint, p256dh, auth_secret, created_at
		FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*PushSubscription
	for rows.Next() {
		s := &PushSubscription{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256DH, &s.Auth, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	for _, s := range subs {
		dec := r.decrypt(s.P256DH, s.Auth)
		s.P256DH, s.Auth = dec[0], dec[1]
	}
	return subs, nil
}

func (r *PushRepository) GetMutedChats(userID string) (map[string]bool, error) {
	var raw []byte
	err := r.db.QueryRow(`SELECT muted_chats FROM notification_settings WHERE user_id = $1`, userID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return map[string]bool{}, nil
	}
	m := make(map[string]bool, len(list))
	for _, id := range list {
		m[id] = true
	}
	return m, nil
}

func (r *PushRepository) SaveSettings(userID string, mutedChats []string) error {
	data, _ := json.Marshal(mutedChats)
	_, err := r.db.Exec(`INSERT INTO notification_settings (user_id, muted_chats, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET muted_chats = $2, updated_at = NOW()`,
		userID, data)
	return err
}
