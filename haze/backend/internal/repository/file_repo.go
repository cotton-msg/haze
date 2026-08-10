package repository

import (
	"database/sql"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type FileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(file *models.File) error {
	query := `INSERT INTO files (message_id, url, mime_type, size, name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	return r.db.QueryRow(query, file.MessageID, file.URL, file.MimeType, file.Size, file.Name, time.Now()).Scan(&file.ID)
}

func (r *FileRepository) FindByID(id string) (*models.File, error) {
	f := &models.File{}
	query := `SELECT id, message_id, url, mime_type, size, name, created_at FROM files WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(&f.ID, &f.MessageID, &f.URL, &f.MimeType, &f.Size, &f.Name, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *FileRepository) FindByMessageID(messageID string) ([]*models.File, error) {
	rows, err := r.db.Query(`SELECT id, message_id, url, mime_type, size, name, created_at FROM files WHERE message_id = $1`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		f := &models.File{}
		if err := rows.Scan(&f.ID, &f.MessageID, &f.URL, &f.MimeType, &f.Size, &f.Name, &f.CreatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (r *FileRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM files WHERE id = $1`, id)
	return err
}
