-- 008_seq.sql
-- Монотонный seq на чат — якорь синхронизации (как у Telegram).

-- Счётчики сообщений per chat.
CREATE TABLE IF NOT EXISTS chat_seq (
    chat_id UUID PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    next_seq BIGINT NOT NULL DEFAULT 1
);

-- Уже существующие сообщения получают seq по порядку внутри чата.
DO $$
DECLARE
    chat_row RECORD;
    idx BIGINT;
BEGIN
    FOR chat_row IN SELECT DISTINCT chat_id FROM messages
    LOOP
        idx := 1;
        FOR msg_row IN SELECT id FROM messages
                      WHERE chat_id = chat_row.chat_id
                      ORDER BY created_at ASC, id ASC
        LOOP
            UPDATE messages SET seq = idx WHERE id = msg_row.id;
            idx := idx + 1;
        END LOOP;
        INSERT INTO chat_seq (chat_id, next_seq) VALUES (chat_row.chat_id, idx)
        ON CONFLICT (chat_id) DO UPDATE SET next_seq = EXCLUDED.next_seq;
    END LOOP;
END $$;

-- Индекс для выборки истории/синка по seq.
CREATE INDEX IF NOT EXISTS idx_messages_seq ON messages(chat_id, seq DESC);
