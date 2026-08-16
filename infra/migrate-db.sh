#!/usr/bin/env bash
# One-time migration (issue #1031): dump the full Supabase-managed database
# (app schemas + Supabase Auth's own `auth` schema) and restore it into the
# self-hosted Postgres stood up by `tofu apply` (see postgres-compose.yml /
# null_resource.postgres in main.tf). NOT Tofu-managed — this needs the
# *source* DB_DSN, which Tofu has no reason to hold, and is inherently a
# one-off operation, not repeatable infra. Safe to re-run: it only reads
# from the source; Supabase's own database is never modified.
#
# The dump never touches disk anywhere (not locally, not on the VPS) — it
# streams straight from pg_dump, through ssh, into pg_restore inside the
# container.
set -euo pipefail

SOURCE_DSN="${SOURCE_DB_DSN:?set SOURCE_DB_DSN to the Supabase pooler DSN}"
DEPLOY_HOST="${DEPLOY_HOST:?set DEPLOY_HOST to deploy@<vps-ip>}"
CONTAINER="${PG_CONTAINER:?set PG_CONTAINER to the running postgres container name, see: ssh $DEPLOY_HOST docker ps}"

pg_dump --format=custom --dbname="$SOURCE_DSN" |
  ssh "$DEPLOY_HOST" \
    "docker exec -i '$CONTAINER' pg_restore --no-owner --no-privileges --username=postgres --dbname=postgres --verbose"
