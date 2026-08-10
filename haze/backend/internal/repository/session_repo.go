package repository

import (
	"database/sql"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(session *models.Session) error {
	query := `INSERT INTO sessions (user_id, refresh_token, user_agent, ip, is_revoked, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	return r.db.QueryRow(query, session.UserID, session.RefreshToken, session.UserAgent,
		session.IP, false, session.ExpiresAt, time.Now()).Scan(&session.ID)
}

func (r *SessionRepository) FindByRefreshToken(token string) (*models.Session, error) {
	session := &models.Session{}
	query := `SELECT id, user_id, refresh_token, user_agent, ip, is_revoked, expires_at, created_at
		FROM sessions WHERE refresh_token = $1`
	err := r.db.QueryRow(query, token).Scan(&session.ID, &session.UserID, &session.RefreshToken,
		&session.UserAgent, &session.IP, &session.IsRevoked, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *SessionRepository) Revoke(id string) error {
	_, err := r.db.Exec(`UPDATE sessions SET is_revoked = TRUE WHERE id = $1`, id)
	return err
}

func (r *SessionRepository) RevokeByUserID(userID string) error {
	_, err := r.db.Exec(`UPDATE sessions SET is_revoked = TRUE WHERE user_id = $1 AND is_revoked = FALSE`, userID)
	return err
}

func (r *SessionRepository) DeleteExpired() error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}
