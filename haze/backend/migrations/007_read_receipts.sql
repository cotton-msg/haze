-- 007_read_receipts.sql
-- Миграция: read receipts (последнее прочитанное сообщение на пользователя)

CREATE TABLE IF NOT EXISTS chat_read (
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_chat_read_user ON chat_read(user_id);
