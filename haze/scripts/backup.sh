#!/usr/bin/env bash
set -euo pipefail

# Запуск из любого места: пути резолвим от корня проекта haze/
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HAZE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$HAZE_ROOT"

BACKUP_DIR="$HAZE_ROOT/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_URL="${DATABASE_URL:-postgres://haze:haze@localhost:5432/haze?sslmode=disable}"

mkdir -p "$BACKUP_DIR"

echo "Backing up PostgreSQL..."
pg_dump "$DB_URL" | gzip > "$BACKUP_DIR/haze_db_$TIMESTAMP.sql.gz"

echo "Backing up media..."
tar czf "$BACKUP_DIR/haze_media_$TIMESTAMP.tar.gz" -C "$HAZE_ROOT/backend/media" . 2>/dev/null || true

echo "Backup saved to $BACKUP_DIR/haze_db_$TIMESTAMP.sql.gz"
echo "Done!"
