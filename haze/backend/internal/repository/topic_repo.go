package repository

import (
	"database/sql"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type TopicRepository struct {
	db *sql.DB
}

func NewTopicRepository(db *sql.DB) *TopicRepository {
	return &TopicRepository{db: db}
}

func (r *TopicRepository) Create(topic *models.Topic) error {
	query := `INSERT INTO topics (chat_id, title, is_pinned, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	return r.db.QueryRow(query, topic.ChatID, topic.Title, topic.IsPinned, time.Now()).Scan(&topic.ID)
}

func (r *TopicRepository) FindByChatID(chatID string) ([]*models.Topic, error) {
	rows, err := r.db.Query(`SELECT id, chat_id, title, is_pinned, message_count, last_message_at, created_at FROM topics WHERE chat_id = $1 ORDER BY is_pinned DESC, created_at ASC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []*models.Topic
	for rows.Next() {
		t := &models.Topic{}
		if err := rows.Scan(&t.ID, &t.ChatID, &t.Title, &t.IsPinned, &t.MessageCount, &t.LastMessageAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, nil
}

func (r *TopicRepository) Update(topic *models.Topic) error {
	query := `UPDATE topics SET title=$1, is_pinned=$2 WHERE id=$3`
	_, err := r.db.Exec(query, topic.Title, topic.IsPinned, topic.ID)
	return err
}

func (r *TopicRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM topics WHERE id = $1`, id)
	return err
}

type FolderRepository struct {
	db *sql.DB
}

func NewFolderRepository(db *sql.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

func (r *FolderRepository) Create(folder *models.ChatFolder) error {
	query := `INSERT INTO chat_folders (user_id, name, icon, emoji, position) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return r.db.QueryRow(query, folder.UserID, folder.Name, folder.Icon, folder.Emoji, folder.Position).Scan(&folder.ID)
}

func (r *FolderRepository) FindByUserID(userID string) ([]*models.ChatFolder, error) {
	rows, err := r.db.Query(`SELECT id, user_id, name, icon, emoji, position FROM chat_folders WHERE user_id = $1 ORDER BY position`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []*models.ChatFolder
	for rows.Next() {
		f := &models.ChatFolder{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.Name, &f.Icon, &f.Emoji, &f.Position); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, nil
}

func (r *FolderRepository) AddChat(folderID, chatID string) error {
	_, err := r.db.Exec(`INSERT INTO chat_folder_chats (folder_id, chat_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, folderID, chatID)
	return err
}

func (r *FolderRepository) RemoveChat(folderID, chatID string) error {
	_, err := r.db.Exec(`DELETE FROM chat_folder_chats WHERE folder_id = $1 AND chat_id = $2`, folderID, chatID)
	return err
}

func (r *FolderRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM chat_folders WHERE id = $1`, id)
	return err
}

func (r *FolderRepository) Update(folder *models.ChatFolder) error {
	query := `UPDATE chat_folders SET name=$1, icon=$2, emoji=$3, position=$4 WHERE id=$5 AND user_id=$6`
	_, err := r.db.Exec(query, folder.Name, folder.Icon, folder.Emoji, folder.Position, folder.ID, folder.UserID)
	return err
}
