package repository

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type BotRepository struct {
	db *sql.DB
}

func NewBotRepository(db *sql.DB) *BotRepository {
	return &BotRepository{db: db}
}

// Create создаёт бота. Бот является и пользователем (users), чтобы сообщения
// и членство в чатах корректно ссылались по FK. id бота == id пользователя.
func (r *BotRepository) Create(ownerID, token, username, name, description string) (string, error) {
	id := uuid.NewString()

	tx, err := r.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO users (id, ssa_id, username, display_name, bio, role, is_bot)
		VALUES ($1, $2, $3, $4, $5, 'bot', TRUE)`,
		id, "bot:"+username, username, name, description); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO bots (id, owner_id, token, username, name, description)
		VALUES ($1, $2, $3, $4, $5, $6)`, id, ownerID, token, username, name, description); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (r *BotRepository) FindByToken(token string) (map[string]interface{}, error) {
	var id, ownerID, username, name, description string
	var isPremium bool
	err := r.db.QueryRow(`SELECT id, owner_id, username, name, description, is_premium FROM bots WHERE token = $1`, token).
		Scan(&id, &ownerID, &username, &name, &description, &isPremium)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "owner_id": ownerID, "username": username,
		"name": name, "description": description, "is_premium": isPremium,
	}, nil
}

func (r *BotRepository) FindByID(id string) (map[string]interface{}, error) {
	var ownerID, username, name, description, webhook string
	var isPremium bool
	err := r.db.QueryRow(`SELECT id, owner_id, username, name, description, is_premium, webhook_url
		FROM bots WHERE id = $1`, id).
		Scan(&id, &ownerID, &username, &name, &description, &isPremium, &webhook)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "owner_id": ownerID, "username": username,
		"name": name, "description": description, "is_premium": isPremium,
		"webhook_url": webhook,
	}, nil
}

func (r *BotRepository) SetWebhook(botID, url string) error {
	_, err := r.db.Exec(`UPDATE bots SET webhook_url = $2 WHERE id = $1`, botID, url)
	return err
}

func (r *BotRepository) GetWebhook(botID string) (string, error) {
	var url string
	err := r.db.QueryRow(`SELECT COALESCE(webhook_url, '') FROM bots WHERE id = $1`, botID).Scan(&url)
	if err != nil {
		return "", err
	}
	return url, nil
}

func (r *BotRepository) List() ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`SELECT id, owner_id, username, name, description, is_premium, webhook_url FROM bots ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []map[string]interface{}
	for rows.Next() {
		var id, ownerID, username, name, description, webhook string
		var isPremium bool
		if err := rows.Scan(&id, &ownerID, &username, &name, &description, &isPremium, &webhook); err != nil {
			return nil, err
		}
		bots = append(bots, map[string]interface{}{
			"id": id, "owner_id": ownerID, "username": username,
			"name": name, "description": description, "is_premium": isPremium,
			"webhook_url": webhook,
		})
	}
	return bots, nil
}

var ErrBotNotFound = errors.New("bot not found")

type BotCommandRepository struct {
	db *sql.DB
}

func NewBotCommandRepository(db *sql.DB) *BotCommandRepository {
	return &BotCommandRepository{db: db}
}

func (r *BotCommandRepository) SetCommands(botID string, commands []map[string]string) error {
	r.db.Exec(`DELETE FROM bot_commands WHERE bot_id = $1`, botID)
	for _, cmd := range commands {
		_, err := r.db.Exec(`INSERT INTO bot_commands (bot_id, command, description) VALUES ($1, $2, $3)`,
			botID, cmd["command"], cmd["description"])
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *BotCommandRepository) GetCommands(botID string) ([]map[string]string, error) {
	rows, err := r.db.Query(`SELECT command, description FROM bot_commands WHERE bot_id = $1 ORDER BY command`, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []map[string]string
	for rows.Next() {
		var cmd, desc string
		if err := rows.Scan(&cmd, &desc); err != nil {
			return nil, err
		}
		cmds = append(cmds, map[string]string{"command": cmd, "description": desc})
	}
	return cmds, nil
}

type BadgeRepository struct {
	db *sql.DB
}

func NewBadgeRepository(db *sql.DB) *BadgeRepository {
	return &BadgeRepository{db: db}
}

func (r *BadgeRepository) Assign(userID, badgeType, assignedBy string) error {
	_, err := r.db.Exec(`INSERT INTO user_badges (user_id, badge_type, assigned_by) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, badge_type) DO NOTHING`, userID, badgeType, assignedBy)
	return err
}

func (r *BadgeRepository) Remove(userID, badgeType string) error {
	_, err := r.db.Exec(`DELETE FROM user_badges WHERE user_id = $1 AND badge_type = $2`, userID, badgeType)
	return err
}

func (r *BadgeRepository) GetByUser(userID string) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`SELECT badge_type, assigned_at FROM user_badges WHERE user_id = $1 ORDER BY assigned_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var badges []map[string]interface{}
	for rows.Next() {
		var badgeType string
		var assignedAt time.Time
		if err := rows.Scan(&badgeType, &assignedAt); err != nil {
			return nil, err
		}
		badges = append(badges, map[string]interface{}{
			"type": badgeType, "assigned_at": assignedAt,
		})
	}
	return badges, nil
}

func (r *BadgeRepository) GetAll() ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`SELECT ub.user_id, ub.badge_type, ub.assigned_at, u.username
		FROM user_badges ub JOIN users u ON ub.user_id = u.id ORDER BY ub.assigned_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var badges []map[string]interface{}
	for rows.Next() {
		var userID, badgeType, username string
		var assignedAt time.Time
		if err := rows.Scan(&userID, &badgeType, &assignedAt, &username); err != nil {
			return nil, err
		}
		badges = append(badges, map[string]interface{}{
			"user_id": userID, "type": badgeType,
			"assigned_at": assignedAt, "username": username,
		})
	}
	return badges, nil
}
