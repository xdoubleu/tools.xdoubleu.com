#!/usr/bin/env bash
# Validate the Prometheus and OpenTofu configs pre-merge: infra-apply runs
# after merge and doesn't fail on a malformed prometheus.yml.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
infra_dir="$repo_root/infra"

# Keep in step with infra/prometheus-compose.yml's image tag.
prom_image="prom/prometheus:v3.16.0"

echo "==> promtool check config (infra/prometheus.yml)"
# promtool fails if the `api` job's credentials_file is missing; mount a stub.
docker run --rm \
	--entrypoint promtool \
	-v "$infra_dir/prometheus.yml:/prometheus.yml:ro" \
	-v /dev/null:/etc/prometheus/ingest_secret:ro \
	"$prom_image" check config /prometheus.yml

echo "==> tofu fmt -check (infra/)"
tofu -chdir="$infra_dir" fmt -check -diff

# -backend=false: no remote state or credentials needed.
echo "==> tofu init -backend=false (infra/)"
tofu -chdir="$infra_dir" init -backend=false -input=false >/dev/null

echo "==> tofu validate (infra/)"
tofu -chdir="$infra_dir" validate

# An apply must never reboot the server: it is not managed here (var.server_id),
# no script may reboot it, and unattended-upgrades must not.
echo "==> no reboot path in infra/"
if grep -nE '^resource "hcloud_server"' "$infra_dir"/*.tf; then
	echo "infra must not manage hcloud_server: a replace reboots production" >&2
	exit 1
fi
if grep -nE '(^|[^[:alnum:]_-])(reboot|shutdown|poweroff)([[:space:]]|$)|systemctl[[:space:]]+(reboot|poweroff)|Automatic-Reboot[^;]*"true"' \
	"$infra_dir"/*.sh "$infra_dir"/*.tf | grep -vE '^[^:]+:[0-9]+:[[:space:]]*#'; then
	echo "infra scripts must not reboot the server" >&2
	exit 1
fi

echo "ok: prometheus.yml + OpenTofu config valid"
