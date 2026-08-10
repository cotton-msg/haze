#!/usr/bin/env bash
set -euo pipefail

# Запуск из любого места: пути резолвим от корня проекта haze/
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HAZE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$HAZE_ROOT"

ENV=${1:-development}
COMPOSE_FILE="docker-compose.yml"

if [ "$ENV" = "production" ]; then
    COMPOSE_FILE="docker-compose.prod.yml"
fi

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "Error: $COMPOSE_FILE not found in $HAZE_ROOT" >&2
    exit 1
fi

echo "Deploying Haze ($ENV) from $HAZE_ROOT..."

docker compose -f "$COMPOSE_FILE" pull
docker compose -f "$COMPOSE_FILE" up -d --remove-orphans

echo "Waiting for health checks..."
sleep 5

SERVICES="gateway auth chat media call bot notification search premium"
for svc in $SERVICES; do
    if docker compose -f "$COMPOSE_FILE" ps "$svc" | grep -q "Up"; then
        echo "  ✓ $svc is running"
    else
        echo "  ✗ $svc failed"
        docker compose -f "$COMPOSE_FILE" logs "$svc" --tail=20
        exit 1
    fi
done

echo "Deployment complete!"
