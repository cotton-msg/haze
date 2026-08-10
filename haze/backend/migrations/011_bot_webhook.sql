-- Боты: вебхуки и признак бота у пользователей.
-- Бот хранится и в users (для FK сообщений/участников), и в bots (id совпадает).
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_bot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE bots ADD COLUMN IF NOT EXISTS webhook_url TEXT NOT NULL DEFAULT '';
