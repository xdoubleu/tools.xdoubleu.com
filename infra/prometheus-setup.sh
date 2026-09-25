# Host-side setup for null_resource.prometheus (main.tf), run from
# /home/deploy/prometheus. Hashed into its triggers; keep it out of remote-exec.
set -euo pipefail

# Host-specific docker gid for prometheus-compose.yml's `group_add`.
DOCKER_GID=$(getent group docker | cut -d: -f3)
[ -n "$DOCKER_GID" ] || { echo 'no docker group on host'; exit 1; }
export DOCKER_GID

docker compose up -d --remove-orphans

# `up -d` doesn't recreate on a prometheus.yml content change, so always
# SIGHUP to reload the config.
docker compose kill -s HUP prometheus