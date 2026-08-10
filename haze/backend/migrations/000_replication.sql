-- 000_replication.sql
-- Роль для streaming-реплики. Выполняется при первой инициализации БД.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'rep_user') THEN
        CREATE ROLE rep_user WITH REPLICATION LOGIN PASSWORD 'haze';
    END IF;
END $$;
