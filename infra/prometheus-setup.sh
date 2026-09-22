# Host-side setup for the Prometheus compose accessory (null_resource.prometheus
# in main.tf). Runs on the VPS from /home/deploy/prometheus. This file is
# hashed into the resource's `triggers` — an edit here forces the provisioners
# to re-run on the next apply (see main.tf's setup_hash comment for why the
# script must not be inlined back into remote-exec).
set -euo pipefail

# DOCKER_GID feeds prometheus-compose.yml's `group_add` so Prometheus, which
# runs as `nobody`, can read the Docker socket it needs for the api/web
# docker_sd_configs (issue #1554). The gid is host-specific, so it is resolved
# here rather than hardcoded in the compose file; failing loudly beats
# silently starting a Prometheus whose service discovery can never work.
DOCKER_GID=$(getent group docker | cut -d: -f3)
[ -n "$DOCKER_GID" ] || { echo 'no docker group on host'; exit 1; }
export DOCKER_GID

docker compose up -d --remove-orphans

# `docker compose up -d` only recreates a container when the *compose service
# definition* changes (image, env, volumes list, etc.) — it has no way to
# notice that prometheus.yml's *content* changed on disk via the file
# provisioners, since the bind-mount declaration itself is unchanged. A
# prometheus.yml-only edit (e.g. adding a new scrape job) therefore
# re-uploaded the file but left the already-running Prometheus process on its
# old, already-loaded in-memory config indefinitely — exactly what happened to
# the `grafana` scrape job added after Prometheus was already running (issue
# #1717; the `api`/`web` docker_sd_configs migration only ever worked because
# that change also touched the compose file's Docker-socket mount, which did
# force a recreate). Prometheus supports SIGHUP as a live, zero-downtime
# config reload, so send it unconditionally after `up -d` — safe whether the
# container was just freshly created (reloading the config it already booted
# with) or was already running (this is the only thing that ever applies a
# prometheus.yml-only change).
docker compose kill -s HUP prometheus