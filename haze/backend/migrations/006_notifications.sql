-- 006_notifications.sql
-- Миграция: push-подписки и настройки уведомлений

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth_secret TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, endpoint)
);

CREATE INDEX idx_push_subscriptions_user_id ON push_subscriptions(user_id);

CREATE TABLE IF NOT EXISTS notification_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    notify_all_chats BOOLEAN NOT NULL DEFAULT TRUE,
    muted_chats JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Столбец для привязки платежей к Stripe-сессиям
ALTER TABLE premium_payments ADD COLUMN IF NOT EXISTS stripe_session_id VARCHAR(255) DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_premium_payments_stripe_session ON premium_payments(stripe_session_id);
