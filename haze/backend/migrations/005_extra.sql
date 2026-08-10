-- 005_extra.sql
-- Миграция: темы, папки, каналы, премиум, боты, темы оформления

CREATE TABLE IF NOT EXISTS topics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    is_pinned BOOLEAN DEFAULT FALSE,
    message_count INT DEFAULT 0,
    last_message_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_topics_chat_id ON topics(chat_id);

CREATE TABLE IF NOT EXISTS chat_folders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    icon VARCHAR(50) DEFAULT '',
    emoji VARCHAR(10) DEFAULT '',
    position INT DEFAULT 0
);

CREATE INDEX idx_chat_folders_user_id ON chat_folders(user_id);

CREATE TABLE IF NOT EXISTS chat_folder_chats (
    folder_id UUID NOT NULL REFERENCES chat_folders(id) ON DELETE CASCADE,
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    PRIMARY KEY (folder_id, chat_id)
);

CREATE TABLE IF NOT EXISTS user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme_id UUID,
    wallpaper_url TEXT,
    font_size INT DEFAULT 16,
    notification_sounds JSONB DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS themes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    author_id UUID REFERENCES users(id) ON DELETE SET NULL,
    is_premium BOOLEAN DEFAULT FALSE,
    colors JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wallpapers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    preview_url TEXT DEFAULT '',
    is_premium BOOLEAN DEFAULT FALSE,
    category VARCHAR(50) DEFAULT 'abstract'
);

CREATE TABLE IF NOT EXISTS premium_plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    duration INT NOT NULL,
    features JSONB DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS premium_subscriptions (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES premium_plans(id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ends_at TIMESTAMPTZ NOT NULL,
    auto_renew BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS premium_payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES premium_plans(id) ON DELETE CASCADE,
    amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(32) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT DEFAULT '',
    is_premium BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_bots_owner_id ON bots(owner_id);

CREATE TABLE IF NOT EXISTS bot_commands (
    bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    command VARCHAR(100) NOT NULL,
    description TEXT DEFAULT '',
    PRIMARY KEY (bot_id, command)
);
