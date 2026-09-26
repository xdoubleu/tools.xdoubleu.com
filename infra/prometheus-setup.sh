#!/usr/bin/env bash
# Host-side setup for null_resource.prometheus (main.tf), run from
# /home/deploy/prometheus. Hashed into its triggers; keep it out of remote-exec.
set -euo pipefail

# Host-specific docker gid for prometheus-compose.yml's `group_add`.
DOCKER_GID=$(getent group docker | cut -d: -f3)
[ -n "$DOCKER_GID" ] || { echo 'no docker group on host'; exit 1; }
export DOCKER_GID

# Persist the gid into `.env` (auto-read by every `docker compose` invocation
# for var interpolation) so the container can be recreated from any context, not
# only this script. prometheus-compose.yml's `group_add` interpolates
# `${DOCKER_GID}`; a plain `docker compose up` without it yields a blank group
# id, which the daemon rejects ("unable to find group"). Idempotent, so it also
# covers a gid change on re-apply.
grep -qxF "DOCKER_GID=$DOCKER_GID" .env 2>/dev/null || echo "DOCKER_GID=$DOCKER_GID" >>.env

docker compose up -d --remove-orphans

# `up -d` doesn't recreate on a prometheus.yml content change, so always
# SIGHUP to reload the config.
docker compose kill -s HUP prometheus

# A systemd unit makes the stack self-heal across reboots (unattended-upgrades
# reboots at 04:00) and survives infra-apply no-ops: it runs `docker compose
# up -d` idempotently on boot and retries on failure, so a stopped/removed
# Prometheus can't leave Grafana dataless between tofu runs. Installing it here
# (not harden.sh) keeps it coupled to the compose project this script manages.
sudo tee /etc/systemd/system/prometheus-compose.service >/dev/null <<'EOF'
[Unit]
Description=Prometheus compose stack (tools.xdoubleu.com metrics)
After=docker.service network-online.target
Requires=docker.service
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
User=deploy
Group=docker
WorkingDirectory=/home/deploy/prometheus
ExecStart=/usr/bin/docker compose up -d --remove-orphans
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now prometheus-compose.service