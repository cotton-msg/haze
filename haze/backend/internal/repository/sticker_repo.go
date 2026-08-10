package repository

import (
	"database/sql"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type StickerRepository struct {
	db *sql.DB
}

func NewStickerRepository(db *sql.DB) *StickerRepository {
	return &StickerRepository{db: db}
}

func (r *StickerRepository) GetPacks() ([]*models.StickerPack, error) {
	rows, err := r.db.Query(`SELECT id, name, is_premium, thumbnail_url FROM sticker_packs ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packs []*models.StickerPack
	for rows.Next() {
		p := &models.StickerPack{}
		if err := rows.Scan(&p.ID, &p.Name, &p.IsPremium, &p.ThumbnailURL); err != nil {
			return nil, err
		}
		packs = append(packs, p)
	}
	return packs, nil
}

func (r *StickerRepository) GetByPack(packID string) ([]*models.Sticker, error) {
	rows, err := r.db.Query(`SELECT id, name, image_url, pack_id FROM stickers WHERE pack_id = $1`, packID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stickers []*models.Sticker
	for rows.Next() {
		s := &models.Sticker{}
		if err := rows.Scan(&s.ID, &s.Name, &s.ImageURL, &s.PackID); err != nil {
			return nil, err
		}
		stickers = append(stickers, s)
	}
	return stickers, nil
}

type ReactionRepository struct {
	db *sql.DB
}

func NewReactionRepository(db *sql.DB) *ReactionRepository {
	return &ReactionRepository{db: db}
}

func (r *ReactionRepository) Upsert(reaction *models.Reaction) error {
	query := `INSERT INTO reactions (message_id, user_id, emoji, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (message_id, user_id, emoji) DO NOTHING RETURNING id`
	return r.db.QueryRow(query, reaction.MessageID, reaction.UserID, reaction.Emoji, reaction.CreatedAt).Scan(&reaction.ID)
}

func (r *ReactionRepository) Delete(messageID, userID, emoji string) error {
	_, err := r.db.Exec(`DELETE FROM reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji)
	return err
}

func (r *ReactionRepository) FindByMessageID(messageID string) ([]*models.Reaction, error) {
	rows, err := r.db.Query(`SELECT id, message_id, user_id, emoji, created_at FROM reactions WHERE message_id = $1`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*models.Reaction
	for rows.Next() {
		re := &models.Reaction{}
		if err := rows.Scan(&re.ID, &re.MessageID, &re.UserID, &re.Emoji, &re.CreatedAt); err != nil {
			return nil, err
		}
		reactions = append(reactions, re)
	}
	return reactions, nil
}

// FindByMessageIDAggregated считает реакции по эмодзи одним запросом.
func (r *ReactionRepository) FindByMessageIDAggregated(messageID string) ([]*models.ReactionCount, error) {
	rows, err := r.db.Query(`SELECT emoji, COUNT(*) FROM reactions WHERE message_id = $1 GROUP BY emoji ORDER BY COUNT(*) DESC, emoji`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []*models.ReactionCount
	for rows.Next() {
		c := &models.ReactionCount{}
		if err := rows.Scan(&c.Emoji, &c.Count); err != nil {
			return nil, err
		}
		counts = append(counts, c)
	}
	return counts, nil
}

func (r *StickerRepository) CreatePack(pack *models.StickerPack) error {
	query := `INSERT INTO sticker_packs (name, is_premium, thumbnail_url) VALUES ($1, $2, $3) RETURNING id`
	return r.db.QueryRow(query, pack.Name, pack.IsPremium, pack.ThumbnailURL).Scan(&pack.ID)
}

func (r *StickerRepository) UpdatePack(pack *models.StickerPack) error {
	query := `UPDATE sticker_packs SET name=$1, is_premium=$2, thumbnail_url=$3 WHERE id=$4`
	_, err := r.db.Exec(query, pack.Name, pack.IsPremium, pack.ThumbnailURL, pack.ID)
	return err
}

func (r *StickerRepository) DeletePack(packID string) error {
	_, err := r.db.Exec(`DELETE FROM sticker_packs WHERE id = $1`, packID)
	return err
}

func (r *StickerRepository) CreateSticker(sticker *models.Sticker) error {
	query := `INSERT INTO stickers (pack_id, name, image_url) VALUES ($1, $2, $3) RETURNING id`
	return r.db.QueryRow(query, sticker.PackID, sticker.Name, sticker.ImageURL).Scan(&sticker.ID)
}

func (r *StickerRepository) DeleteSticker(stickerID string) error {
	_, err := r.db.Exec(`DELETE FROM stickers WHERE id = $1`, stickerID)
	return err
}
