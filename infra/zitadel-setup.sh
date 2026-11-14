#!/usr/bin/env bash
# Runs on the VPS in /home/deploy/zitadel (uploaded by null_resource.zitadel).
# Idempotent: creates Zitadel's database and role, starts it, and routes
# $ZITADEL_HOST to it through kamal-proxy.
set -euo pipefail

cd "$(dirname "$0")"
set -a
# shellcheck disable=SC1091
. ./.env
set +a

# Not a superuser; Zitadel owns only its own database.
pg() { docker exec -i postgres psql -v ON_ERROR_STOP=1 --username=postgres --dbname=postgres "$@"; }

if [ -z "$(pg -tAc "SELECT 1 FROM pg_roles WHERE rolname = 'zitadel'")" ]; then
	pg -c "CREATE ROLE zitadel LOGIN PASSWORD '${ZITADEL_DATABASE_POSTGRES_USER_PASSWORD}'"
else
	pg -c "ALTER ROLE zitadel PASSWORD '${ZITADEL_DATABASE_POSTGRES_USER_PASSWORD}'"
fi
if [ -z "$(pg -tAc "SELECT 1 FROM pg_database WHERE datname = 'zitadel'")" ]; then
	pg -c "CREATE DATABASE zitadel OWNER zitadel"
fi

# Prometheus, Grafana and the api never need this database; keep it private.
pg -c "REVOKE CONNECT ON DATABASE zitadel FROM PUBLIC"

if [ "${COMPOSE_PULL:-1}" = 1 ]; then
	{ timeout 300 docker compose pull --quiet || echo 'docker compose pull failed; keeping the local images' >&2; }
fi
docker compose up -d --remove-orphans

# kamal-proxy only exists once Kamal has set the host up; the route is applied
# on the next apply after that. Cert issuance needs a DNS A record for the host.
if docker ps --format '{{.Names}}' | grep -qx kamal-proxy; then
	docker exec kamal-proxy kamal-proxy deploy zitadel \
		--target zitadel:8080 --host "$ZITADEL_HOST" --tls \
		--health-check-path /debug/healthz --health-check-interval 10s
else
	echo 'kamal-proxy is not running; skipping the Zitadel route' >&2
fi
