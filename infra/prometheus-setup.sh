#!/usr/bin/env bash
# Host-side setup for null_resource.prometheus (main.tf), run from
# /home/deploy/prometheus. Hashed into its triggers; keep it out of remote-exec.
set -euo pipefail

# Serializes with prometheus-compose.service's timer runs (below): two
# concurrent `compose up`s can race on a container recreate.
exec 9>.compose.lock
flock 9

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

# Same pull as main.tf's local.compose_up: a failed pull keeps the local images.
# The timer below never pulls.
timeout 300 docker compose pull --quiet || echo 'docker compose pull failed; keeping the local images' >&2
docker compose up -d --remove-orphans

# `up -d` doesn't recreate on a prometheus.yml content change, so always
# SIGHUP to reload the config. Non-fatal: a container that isn't running has
# nothing to reload, and failing here would make infra-apply rebuild the VPS
# from its snapshot.
docker compose kill -s HUP prometheus || echo 'prometheus not running; skipped config reload' >&2

flock -u 9

# The timer re-runs `docker compose up -d` every 2 minutes (and on boot), which
# starts an exited or removed container whatever stopped it: a reboot
# (unattended-upgrades reboots at 04:00), a kill, or a failed recreate.
# Installed here (not harden.sh) to stay coupled to this compose project.
sudo tee /etc/systemd/system/prometheus-compose.service >/dev/null <<'EOF'
[Unit]
Description=Ensure the Prometheus compose stack (tools.xdoubleu.com metrics) is up
After=docker.service network-online.target
Requires=docker.service
Wants=network-online.target

[Service]
Type=oneshot
User=deploy
Group=docker
WorkingDirectory=/home/deploy/prometheus
ExecStart=/usr/bin/flock /home/deploy/prometheus/.compose.lock /usr/bin/docker compose up -d --remove-orphans
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
sudo tee /etc/systemd/system/prometheus-compose.timer >/dev/null <<'EOF'
[Unit]
Description=Re-ensure the Prometheus compose stack every 2 minutes

[Timer]
OnBootSec=1min
OnUnitActiveSec=2min

[Install]
WantedBy=timers.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable prometheus-compose.service
# Restart, not start: `start` is a no-op on a unit that is still active.
sudo systemctl restart prometheus-compose.service
sudo systemctl enable --now prometheus-compose.timer
