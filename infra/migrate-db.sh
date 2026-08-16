#!/usr/bin/env bash
# One-time migration (issue #1031): restore an existing pg_dump (custom
# format) of the Supabase-managed database into the self-hosted Postgres
# stood up by `tofu apply` (see postgres-compose.yml / null_resource.postgres
# in main.tf). NOT Tofu-managed — inherently a one-off operation, not
# repeatable infra. Safe to re-run: the dump file is never modified.
#
# The dump file never touches disk on the VPS — it streams straight through
# ssh into pg_restore inside the container.
set -euo pipefail

DUMP_FILE="${1:?usage: $0 <path-to-pg_dump-file>}"
DEPLOY_HOST="${DEPLOY_HOST:?set DEPLOY_HOST to deploy@<vps-ip>}"
CONTAINER="${PG_CONTAINER:?set PG_CONTAINER to the running postgres container name, see: ssh $DEPLOY_HOST docker ps}"

ssh "$DEPLOY_HOST" \
  "docker exec -i '$CONTAINER' pg_restore --no-owner --no-privileges --username=postgres --dbname=postgres --verbose" \
  <"$DUMP_FILE"
