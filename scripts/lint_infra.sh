#!/usr/bin/env bash
# Validate the Prometheus scrape config and the OpenTofu config before merge
# (issue #1561).
#
# main.yml's infra-apply job is not a validation gate: it runs `tofu apply`
# after merge on main, and it would not fail on a malformed prometheus.yml
# anyway — null_resource.prometheus uploads the file and runs
# `docker compose up -d`, which exits 0 while Prometheus crash-loops on the
# bad config. #1554 is what a silently-wrong scrape config costs.
#
# Run via `make lint/infra`; also run by main.yml's infra-lint job.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
infra_dir="$repo_root/infra"

# Pinned to the same image tag infra/prometheus-compose.yml deploys, so the
# checking binary is the version that will actually parse this file. Keep the
# two in step when bumping Prometheus.
prom_image="prom/prometheus:v3.1.0"

echo "==> promtool check config (infra/prometheus.yml)"
# The `web` scrape job's authorization.credentials_file (issue #1555) points
# at a path Tofu writes on the VPS, not in this repo — `promtool check config`
# fails if that file is missing, so mount an empty stand-in at that path.
docker run --rm \
	--entrypoint promtool \
	-v "$infra_dir/prometheus.yml:/prometheus.yml:ro" \
	-v /dev/null:/etc/prometheus/web_ingest_secret:ro \
	"$prom_image" check config /prometheus.yml

echo "==> tofu fmt -check (infra/)"
tofu -chdir="$infra_dir" fmt -check -diff

# -backend=false: validation needs the providers, not the remote state, so
# this deliberately does not touch backend.hcl or need any credentials.
echo "==> tofu init -backend=false (infra/)"
tofu -chdir="$infra_dir" init -backend=false -input=false >/dev/null

echo "==> tofu validate (infra/)"
tofu -chdir="$infra_dir" validate

echo "ok: prometheus.yml + OpenTofu config valid"
