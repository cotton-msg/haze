package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

type ChatRepository struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Create(chat *models.Chat) error {
	query := `INSERT INTO chats (type, title, avatar, description, is_pinned, pinned_message_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	return r.db.QueryRow(query, chat.Type, chat.Title, chat.Avatar, chat.Description,
		chat.IsPinned, chat.PinnedMessageID, time.Now(), time.Now()).Scan(&chat.ID)
}

func (r *ChatRepository) FindByID(id string) (*models.Chat, error) {
	chat := &models.Chat{}
	query := `SELECT id, type, title, avatar, description, is_pinned, pinned_message_id, created_at, updated_at
		FROM chats WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(&chat.ID, &chat.Type, &chat.Title, &chat.Avatar,
		&chat.Description, &chat.IsPinned, &chat.PinnedMessageID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return chat, nil
}

func (r *ChatRepository) FindByUserID(userID string) ([]*models.Chat, error) {
	query := `SELECT
			c.id, c.type, c.title, c.avatar, c.description, c.is_pinned, c.pinned_message_id, c.created_at, c.updated_at,
			COALESCE((SELECT m.content FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1), '') AS last_message,
			(SELECT COUNT(*) FROM messages m
				WHERE m.chat_id = c.id AND m.sender_id <> $1
				  AND m.created_at > COALESCE(cr.last_read_at, '1970-01-01')) AS unread_count
		FROM chats c
		JOIN chat_members cm ON c.id = cm.chat_id
		LEFT JOIN chat_read cr ON cr.chat_id = c.id AND cr.user_id = $1
		WHERE cm.user_id = $1
		ORDER BY c.updated_at DESC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*models.Chat
	for rows.Next() {
		chat := &models.Chat{}
		if err := rows.Scan(&chat.ID, &chat.Type, &chat.Title, &chat.Avatar,
			&chat.Description, &chat.IsPinned, &chat.PinnedMessageID, &chat.CreatedAt, &chat.UpdatedAt,
			&chat.LastMessage, &chat.UnreadCount); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, nil
}

func (r *ChatRepository) AddMember(chatID, userID string, role models.ChatMemberRole) error {
	query := `INSERT INTO chat_members (chat_id, user_id, role, joined_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = $3`
	_, err := r.db.Exec(query, chatID, userID, role, time.Now())
	return err
}

func (r *ChatRepository) RemoveMember(chatID, userID string) error {
	_, err := r.db.Exec(`DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`, chatID, userID)
	return err
}

func (r *ChatRepository) GetMembers(chatID string) ([]*models.ChatMember, error) {
	rows, err := r.db.Query(`SELECT chat_id, user_id, role, joined_at FROM chat_members WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.ChatMember
	for rows.Next() {
		m := &models.ChatMember{}
		if err := rows.Scan(&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *ChatRepository) GetMemberIDs(chatID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT user_id FROM chat_members WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *ChatRepository) IsMember(chatID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id=$1 AND user_id=$2)`, chatID, userID).Scan(&exists)
	return exists, err
}

// FindSavedChat ищет личный чат, где единственный участник — userID
// («Избранное» — чат с собой). Возвращает sql.ErrNoRows, если его нет.
func (r *ChatRepository) FindSavedChat(userID string) (*models.Chat, error) {
	chat := &models.Chat{}
	query := `SELECT c.id, c.type, c.title, c.avatar, c.description, c.is_pinned, c.pinned_message_id, c.created_at, c.updated_at
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id AND cm.user_id = $1
		WHERE c.type = 'personal'
		  AND (SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id) = 1
		LIMIT 1`
	err := r.db.QueryRow(query, userID).Scan(&chat.ID, &chat.Type, &chat.Title, &chat.Avatar,
		&chat.Description, &chat.IsPinned, &chat.PinnedMessageID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return chat, nil
}

// FindOrCreatePersonal ищет личный чат (type=personal) с ровно двумя участниками
// или возвращает новый чат. Используется для «Избранного» (чат с собой).
func (r *ChatRepository) FindPersonalBetween(a, b string) (*models.Chat, error) {
	chat := &models.Chat{}
	query := `SELECT c.id, c.type, c.title, c.avatar, c.description, c.is_pinned, c.pinned_message_id, c.created_at, c.updated_at
		FROM chats c
		JOIN chat_members ma ON ma.chat_id = c.id AND ma.user_id = $1
		JOIN chat_members mb ON mb.chat_id = c.id AND mb.user_id = $2
		WHERE c.type = 'personal'
		  AND (SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id) = 2
		LIMIT 1`
	err := r.db.QueryRow(query, a, b).Scan(&chat.ID, &chat.Type, &chat.Title, &chat.Avatar,
		&chat.Description, &chat.IsPinned, &chat.PinnedMessageID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return chat, nil
}

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(msg *models.Message) error {
	// Идемпотентность: если client_id уже обработан, возвращаем существующее
	// сообщение, не выделяя новый seq и не создавая дубликат.
	if msg.ClientID != "" {
		existing, err := r.FindByClientID(msg.ChatID, msg.ClientID)
		if err == nil {
			*msg = *existing
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
	}

	// Seq выделяется атомарно из счётчика чата: монотонный и без пропусков.
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO chat_seq (chat_id) VALUES ($1)
		ON CONFLICT (chat_id) DO UPDATE SET next_seq = chat_seq.next_seq + 1
		RETURNING next_seq - 1`
	if err := tx.QueryRow(query, msg.ChatID).Scan(&msg.Seq); err != nil {
		return err
	}

	query = `INSERT INTO messages (chat_id, sender_id, seq, client_id, content, type, reply_to, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	if err := tx.QueryRow(query, msg.ChatID, msg.SenderID, msg.Seq, nullString(msg.ClientID), msg.Content, msg.Type,
		msg.ReplyTo, msg.Status, time.Now(), time.Now()).Scan(&msg.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// FindByClientID находит сообщение по (chat_id, client_id) — для дедупликации.
func (r *MessageRepository) FindByClientID(chatID, clientID string) (*models.Message, error) {
	msg := &models.Message{}
	query := `SELECT ` + messageCols + ` FROM messages WHERE chat_id = $1 AND client_id = $2`
	err := scanMessage(r.db.QueryRow(query, chatID, clientID), msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// nullString возвращает NULL для пустых строк (client_id nullable).
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// messageCols — общий список колонок для выборок сообщений.
const messageCols = "id, chat_id, sender_id, seq, client_id, content, type, reply_to, status, edited_at, created_at, updated_at"

func scanMessage(row interface{ Scan(...interface{}) error }, msg *models.Message) error {
	return row.Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Seq, &msg.ClientID, &msg.Content,
		&msg.Type, &msg.ReplyTo, &msg.Status, &msg.EditedAt, &msg.CreatedAt, &msg.UpdatedAt)
}

func (r *MessageRepository) FindByID(id string) (*models.Message, error) {
	msg := &models.Message{}
	query := `SELECT ` + messageCols + ` FROM messages WHERE id = $1`
	err := scanMessage(r.db.QueryRow(query, id), msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (r *MessageRepository) FindByChatID(chatID string, limit, offset int) ([]*models.Message, error) {
	query := `SELECT ` + messageCols + ` FROM messages WHERE chat_id = $1 ORDER BY seq DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(query, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := scanMessage(rows, msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// FindByChatIDAfterSeq возвращает сообщения чата с seq > afterSeq по возрастанию —
// используется для синхронизации устройств.
func (r *MessageRepository) FindByChatIDAfterSeq(chatID string, afterSeq int64, limit int) ([]*models.Message, error) {
	query := `SELECT ` + messageCols + ` FROM messages WHERE chat_id = $1 AND seq > $2 ORDER BY seq ASC LIMIT $3`
	rows, err := r.db.Query(query, chatID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := scanMessage(rows, msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// GetLastSeq возвращает последний seq чата (0, если сообщений ещё нет).
func (r *MessageRepository) GetLastSeq(chatID string) (int64, error) {
	var seq sql.NullInt64
	err := r.db.QueryRow(`SELECT MAX(seq) FROM messages WHERE chat_id = $1`, chatID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return seq.Int64, nil
}

func (r *MessageRepository) Update(msg *models.Message) error {
	query := `UPDATE messages SET content=$1, type=$2, reply_to=$3, status=$4, edited_at=$5, updated_at=$6 WHERE id=$7`
	_, err := r.db.Exec(query, msg.Content, msg.Type, msg.ReplyTo, msg.Status, time.Now(), time.Now(), msg.ID)
	return err
}

func (r *MessageRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM messages WHERE id = $1`, id)
	return err
}

func (r *MessageRepository) UpdateStatus(id string, status models.MessageStatus) error {
	_, err := r.db.Exec(`UPDATE messages SET status=$1, updated_at=$2 WHERE id=$3`, status, time.Now(), id)
	return err
}

// UpdateLastRead записывает время прочтения последнего сообщения чата для пользователя.
func (r *MessageRepository) UpdateLastRead(chatID, userID, upToMessageID string) error {
	query := `INSERT INTO chat_read (chat_id, user_id, last_read_at)
		VALUES ($1, $2, (SELECT created_at FROM messages WHERE id=$3))
		ON CONFLICT (chat_id, user_id) DO UPDATE SET last_read_at = EXCLUDED.last_read_at`
	_, err := r.db.Exec(query, chatID, userID, upToMessageID)
	return err
}

// MarkRead помечает сообщения чата (отправленные не этим пользователем)
// как прочитанные, если они созданы не позже указанного message_id.
func (r *MessageRepository) MarkRead(chatID, readerID, upToMessageID string) error {
	query := `UPDATE messages SET status=$1, updated_at=$2
		WHERE chat_id=$3 AND sender_id<>$4 AND status<>$1
		  AND created_at <= (SELECT created_at FROM messages WHERE id=$5)`
	_, err := r.db.Exec(query, models.MessageStatusRead, time.Now(), chatID, readerID, upToMessageID)
	return err
}

func (r *MessageRepository) GetLastMessage(chatID string) (*models.Message, error) {
	msg := &models.Message{}
	query := `SELECT ` + messageCols + ` FROM messages WHERE chat_id = $1 ORDER BY seq DESC LIMIT 1`
	err := scanMessage(r.db.QueryRow(query, chatID), msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (r *MessageRepository) ListAll(limit, offset int) ([]*models.Message, error) {
	rows, err := r.db.Query(`SELECT `+messageCols+` FROM messages ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		msg := &models.Message{}
		if err := scanMessage(rows, msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// UpsertLinkPreview сохраняет OG-карточку для сообщения (unfurl).
func (r *MessageRepository) UpsertLinkPreview(msgID string, p *models.LinkPreview) error {
	_, err := r.db.Exec(`INSERT INTO message_link_previews (message_id, url, title, description, image, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (message_id) DO UPDATE SET url=EXCLUDED.url, title=EXCLUDED.title,
			description=EXCLUDED.description, image=EXCLUDED.image`, msgID, p.URL, p.Title, p.Description, p.Image)
	return err
}

// FindLinkPreviewsByMessageIDs возвращает превью для списка сообщений.
func (r *MessageRepository) FindLinkPreviewsByMessageIDs(msgIDs []string) (map[string]*models.LinkPreview, error) {
	out := make(map[string]*models.LinkPreview)
	if len(msgIDs) == 0 {
		return out, nil
	}
	args := make([]interface{}, len(msgIDs))
	placeholders := make([]string, len(msgIDs))
	for i, id := range msgIDs {
		args[i] = id
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf(`SELECT message_id, url, title, description, image FROM message_link_previews
		WHERE message_id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p := &models.LinkPreview{}
		var msgID string
		if err := rows.Scan(&msgID, &p.URL, &p.Title, &p.Description, &p.Image); err != nil {
			return nil, err
		}
		out[msgID] = p
	}
	return out, nil
}
