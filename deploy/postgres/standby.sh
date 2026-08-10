#!/bin/sh
# Standby-скрипт для postgres-replica: при пустом data-каталоге делает
# pg_basebackup с primary, затем запускает postgres в hot standby.
set -e

DATA=/var/lib/postgresql/data

if [ ! -s "$DATA/PG_VERSION" ]; then
    echo "[standby] no base backup, running pg_basebackup from postgres"
    rm -rf "$DATA"/*
    until pg_isready -h postgres -U haze; do
        echo "[standby] waiting for primary..."
        sleep 2
    done
    pg_basebackup -h postgres -U rep_user -D "$DATA" -R -X stream -P
    echo "[standby] base backup complete"
fi

echo "[standby] starting postgres in hot standby"
exec postgres -c hot_standby=on -c max_connections=100
