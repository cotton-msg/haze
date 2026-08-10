-- 009_dedup.sql
-- Идемпотентная отправка: клиент генерирует client_id, повторная отправка
-- того же сообщения не создаёт дубликат.

ALTER TABLE messages ADD COLUMN IF NOT EXISTS client_id UUID;

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_client_id ON messages(chat_id, client_id)
    WHERE client_id IS NOT NULL;
