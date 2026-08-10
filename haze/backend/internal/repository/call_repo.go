package repository

import (
	"database/sql"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type CallRepository struct {
	db *sql.DB
}

func NewCallRepository(db *sql.DB) *CallRepository {
	return &CallRepository{db: db}
}

func (r *CallRepository) Create(call *models.Call) error {
	query := `INSERT INTO calls (caller_id, callee_id, type, status, started_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return r.db.QueryRow(query, call.CallerID, call.CalleeID, call.Type, call.Status, time.Now()).Scan(&call.ID)
}

func (r *CallRepository) FindByID(id string) (*models.Call, error) {
	call := &models.Call{}
	query := `SELECT id, caller_id, callee_id, type, status, started_at, ended_at FROM calls WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(&call.ID, &call.CallerID, &call.CalleeID,
		&call.Type, &call.Status, &call.StartedAt, &call.EndedAt)
	if err != nil {
		return nil, err
	}
	return call, nil
}

func (r *CallRepository) UpdateStatus(id string, status models.CallStatus) error {
	now := time.Now()
	if status == models.CallStatusEnded || status == models.CallStatusMissed || status == models.CallStatusRejected {
		_, err := r.db.Exec(`UPDATE calls SET status = $1, ended_at = $2 WHERE id = $3`, status, now, id)
		return err
	}
	_, err := r.db.Exec(`UPDATE calls SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *CallRepository) GetHistory(userID string, limit, offset int) ([]*models.Call, error) {
	query := `SELECT id, caller_id, callee_id, type, status, started_at, ended_at
		FROM calls WHERE caller_id = $1 OR callee_id = $1
		ORDER BY started_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []*models.Call
	for rows.Next() {
		call := &models.Call{}
		if err := rows.Scan(&call.ID, &call.CallerID, &call.CalleeID,
			&call.Type, &call.Status, &call.StartedAt, &call.EndedAt); err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func (r *CallRepository) AddParticipant(callID, userID string) error {
	_, err := r.db.Exec(`INSERT INTO call_participants (call_id, user_id, joined_at) VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, callID, userID, time.Now())
	return err
}

func (r *CallRepository) GetActiveCall(userID string) (*models.Call, error) {
	call := &models.Call{}
	query := `SELECT c.id, c.caller_id, c.callee_id, c.type, c.status, c.started_at, c.ended_at
		FROM calls c WHERE (c.caller_id = $1 OR c.callee_id = $1)
		AND c.status IN ('ringing', 'active') LIMIT 1`
	err := r.db.QueryRow(query, userID).Scan(&call.ID, &call.CallerID, &call.CalleeID,
		&call.Type, &call.Status, &call.StartedAt, &call.EndedAt)
	if err != nil {
		return nil, err
	}
	return call, nil
}
