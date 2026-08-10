package repository

import (
	"database/sql"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	query := `INSERT INTO users (ssa_id, username, email, phone, avatar_url, display_name, bio, status_text, role, is_premium, last_seen_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`
	return r.db.QueryRow(query, user.SSAID, user.Username, user.Email, user.Phone,
		user.AvatarURL, user.DisplayName, user.Bio, user.StatusText, user.Role,
		user.IsPremium, user.LastSeenAt, time.Now(), time.Now()).Scan(&user.ID)
}

func (r *UserRepository) FindBySSAID(ssaID string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, ssa_id, username, email, phone, avatar_url, display_name, bio, status_text, role, is_premium, last_seen_at, created_at, updated_at
		FROM users WHERE ssa_id = $1`
	err := r.db.QueryRow(query, ssaID).Scan(&user.ID, &user.SSAID, &user.Username, &user.Email,
		&user.Phone, &user.AvatarURL, &user.DisplayName, &user.Bio, &user.StatusText,
		&user.Role, &user.IsPremium, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) FindByID(id string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, ssa_id, username, email, phone, avatar_url, display_name, bio, status_text, role, is_premium, last_seen_at, created_at, updated_at
		FROM users WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(&user.ID, &user.SSAID, &user.Username, &user.Email,
		&user.Phone, &user.AvatarURL, &user.DisplayName, &user.Bio, &user.StatusText,
		&user.Role, &user.IsPremium, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) FindByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, ssa_id, username, email, phone, avatar_url, display_name, bio, status_text, role, is_premium, last_seen_at, created_at, updated_at
		FROM users WHERE username = $1`
	err := r.db.QueryRow(query, username).Scan(&user.ID, &user.SSAID, &user.Username, &user.Email,
		&user.Phone, &user.AvatarURL, &user.DisplayName, &user.Bio, &user.StatusText,
		&user.Role, &user.IsPremium, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	query := `UPDATE users SET username=$1, email=$2, phone=$3, avatar_url=$4, display_name=$5, bio=$6, status_text=$7, role=$8, is_premium=$9, last_seen_at=$10, updated_at=$11 WHERE id=$12`
	_, err := r.db.Exec(query, user.Username, user.Email, user.Phone, user.AvatarURL, user.DisplayName, user.Bio, user.StatusText, user.Role, user.IsPremium, user.LastSeenAt, time.Now(), user.ID)
	return err
}

func (r *UserRepository) UpdateLastSeen(userID string) error {
	query := `UPDATE users SET last_seen_at=$1, updated_at=$2 WHERE id=$3`
	_, err := r.db.Exec(query, time.Now(), time.Now(), userID)
	return err
}

func (r *UserRepository) UpdateUsername(userID, username string) error {
	query := `UPDATE users SET username=$1, updated_at=$2 WHERE id=$3`
	_, err := r.db.Exec(query, username, time.Now(), userID)
	return err
}

func (r *UserRepository) UpdateRole(userID, role string) error {
	query := `UPDATE users SET role=$1, updated_at=$2 WHERE id=$3`
	_, err := r.db.Exec(query, role, time.Now(), userID)
	return err
}

func (r *UserRepository) CheckUsername(username string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE username=$1)`, username).Scan(&exists)
	return exists, err
}

func (r *UserRepository) ListAll(limit, offset int) ([]*models.User, error) {
	rows, err := r.db.Query(`SELECT id, ssa_id, username, email, phone, avatar_url, display_name, bio, status_text, role, is_premium, last_seen_at, created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(&user.ID, &user.SSAID, &user.Username, &user.Email,
			&user.Phone, &user.AvatarURL, &user.DisplayName, &user.Bio, &user.StatusText,
			&user.Role, &user.IsPremium, &user.LastSeenAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
