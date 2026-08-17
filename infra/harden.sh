#!/usr/bin/env bash
# Idempotent OS hardening for the Hetzner VPS (issue #1030). Run as root via
# the null_resource remote-exec provisioner in main.tf; safe to re-run.
set -euo pipefail

DEPLOY_PUBLIC_KEYS="$1"

# --- deploy user -----------------------------------------------------------
if ! id deploy &>/dev/null; then
  useradd --create-home --shell /bin/bash --groups sudo deploy
fi

install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
AUTH_KEYS=/home/deploy/.ssh/authorized_keys
touch "$AUTH_KEYS"
while IFS= read -r key; do
  [ -z "$key" ] && continue
  grep -qxF "$key" "$AUTH_KEYS" || echo "$key" >>"$AUTH_KEYS"
done <<<"$DEPLOY_PUBLIC_KEYS"
chmod 600 "$AUTH_KEYS"
chown deploy:deploy "$AUTH_KEYS"

# deploy has no password (SSH is key-only, and useradd leaves the account
# locked), so sudo needs to be passwordless or it's simply unusable.
echo "deploy ALL=(ALL) NOPASSWD:ALL" >/etc/sudoers.d/deploy
chmod 440 /etc/sudoers.d/deploy
visudo -cf /etc/sudoers.d/deploy

# --- Docker ------------------------------------------------------------
if ! command -v docker &>/dev/null; then
  curl -fsSL https://get.docker.com | sh
fi
usermod -aG docker deploy

# --- fail2ban ------------------------------------------------------------
if ! command -v fail2ban-client &>/dev/null; then
  apt-get update -y
  apt-get install -y fail2ban
fi
systemctl enable --now fail2ban

# --- ufw ------------------------------------------------------------
if ! command -v ufw &>/dev/null; then
  apt-get update -y
  apt-get install -y ufw
fi
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# --- sshd hardening ------------------------------------------------------------
# Validate before touching the running daemon, and confirm it's still
# listening after the reload — a bad reload here can permanently lock out
# every SSH path (root already disabled, deploy not yet trusted).
SSHD_CONFIG=/etc/ssh/sshd_config.d/99-hardening.conf
cat >"$SSHD_CONFIG" <<'EOF'
PasswordAuthentication no
PermitRootLogin no
EOF
sshd -t
systemctl reload ssh
sleep 1
ss -tln | grep -q ':22 ' || {
  echo "sshd not listening on :22 after reload, aborting" >&2
  exit 1
}
