#!/usr/bin/env bash
# Starts a throwaway Postgres on 127.0.0.1:5432 for the api tests in a job sandbox
# (no Docker, no root, no running cluster). Trust auth matches DB_DSN's default.
set -euo pipefail

BIN=$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)
DATA=${PGDATA:-/tmp/pgdata}

pg_isready -q -h 127.0.0.1 && exit 0
[ -d "$DATA" ] || "$BIN/initdb" -D "$DATA" -A trust -E UTF8 --locale=C.UTF-8 -U postgres >/dev/null
"$BIN/pg_ctl" -D "$DATA" -w -l "$DATA/server.log" \
  -o "-k /tmp -c listen_addresses=127.0.0.1 -c fsync=off" start >/dev/null
pg_isready -h 127.0.0.1
