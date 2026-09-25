#!/usr/bin/env bash
# Idempotent OS hardening for the Hetzner VPS; run by main.tf's provisioner.
set -euo pipefail

# NEEDRESTART_MODE=a: needrestart's interactive prompt would hang remote-exec.
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a

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

# deploy has no password (key-only SSH), so sudo must be passwordless.
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

# --- unattended-upgrades ------------------------------------------------------------
if ! dpkg -s unattended-upgrades &>/dev/null; then
  apt-get update -y
  apt-get install -y unattended-upgrades
fi

cat >/etc/apt/apt.conf.d/50unattended-upgrades <<'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-WithUsers "true";
Unattended-Upgrade::Automatic-Reboot-Time "04:00";
EOF

cat >/etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF

systemctl enable --now unattended-upgrades

# --- release-upgrade-check timer ------------------------------------------------------------
# unattended-upgrades never runs do-release-upgrade, so this timer reports new
# LTS releases. The script itself is uploaded by main.tf.
cat >/etc/systemd/system/release-upgrade-check.service <<'EOF'
[Unit]
Description=Check for a new Ubuntu LTS release and email if one is available

[Service]
Type=oneshot
ExecStart=/usr/local/bin/release-upgrade-check.sh
EOF

cat >/etc/systemd/system/release-upgrade-check.timer <<'EOF'
[Unit]
Description=Weekly Ubuntu release-upgrade check

[Timer]
OnCalendar=weekly
Persistent=true

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable --now release-upgrade-check.timer

# --- sshd hardening ------------------------------------------------------------
# Validate before reloading and confirm sshd still listens: a bad reload
# locks out every SSH path.
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
