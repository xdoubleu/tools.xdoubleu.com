#!/usr/bin/env bash
# Idempotent OS hardening for the Hetzner VPS (issue #1030). Run as root via
# the null_resource remote-exec provisioner in main.tf; safe to re-run.
set -euo pipefail

DEPLOY_PUBLIC_KEY="$1"

# --- deploy user -----------------------------------------------------------
if ! id deploy &>/dev/null; then
  useradd --create-home --shell /bin/bash --groups sudo deploy
fi

install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
AUTH_KEYS=/home/deploy/.ssh/authorized_keys
touch "$AUTH_KEYS"
grep -qxF "$DEPLOY_PUBLIC_KEY" "$AUTH_KEYS" || echo "$DEPLOY_PUBLIC_KEY" >>"$AUTH_KEYS"
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
SSHD_CONFIG=/etc/ssh/sshd_config.d/99-hardening.conf
cat >"$SSHD_CONFIG" <<'EOF'
PasswordAuthentication no
PermitRootLogin no
EOF
systemctl reload ssh
