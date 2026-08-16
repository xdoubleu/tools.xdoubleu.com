# infra

OpenTofu config for the Hetzner VPS (issue #1030) that will host the
self-hosted stack (app + Postgres + GoTrue, replacing DO App Platform +
Supabase — see #1029). Manages the firewall and OS-level hardening only; the
server itself is created manually.

Tofu is run locally, not in CI — this is one-time/rare provisioning, not a
recurring deploy.

## One-time setup

1. **Create the server** in the [Hetzner Console](https://console.hetzner.cloud/):
   New server → location `Falkenstein (fsn1)` → image `Ubuntu 24.04` → type
   `CX23` (2 vCPU / 4 GB, same tier as `CX22` — use whichever of the two is
   available in your region) → paste your SSH public key under "SSH keys"
   (needed so this config's hardening step can SSH in as root before the
   `deploy` user exists) → create. Note the server ID and public IPv4 shown
   after creation.
2. **Get an API token**: Hetzner Console → Security → API Tokens → generate
   (read+write).
3. Install OpenTofu (`brew install opentofu` or see
   [opentofu.org](https://opentofu.org/docs/intro/install/)).
4. **Load your SSH key into `ssh-agent`** — the hardening provisioner
   connects via the agent (`ssh-add --apple-use-keychain ~/.ssh/<key>`),
   since it can't read a passphrase-protected key file directly.

## Apply

```bash
cd infra
export HCLOUD_TOKEN=<your token>
tofu init
tofu plan \
  -var hcloud_token="$HCLOUD_TOKEN" \
  -var server_id=<id> \
  -var server_ip=<ip> \
  -var deploy_ssh_public_key="$(cat ~/.ssh/<key>.pub)"
tofu apply <same -var flags as plan>
```

This attaches the firewall (22/80/443 only) and runs `harden.sh` over SSH:
creates a non-root `deploy` user (passwordless sudo + docker groups, your
public key authorized), installs Docker, enables `fail2ban`, configures
`ufw`, and disables root/password SSH login.

`harden.sh` is idempotent — re-running `tofu apply` after editing it (or with
no changes at all) is safe.

## Verify

```bash
ssh deploy@<ip>                    # should work, key auth only
ssh root@<ip>                      # should be rejected
sudo ufw status                    # on the box: only 22/80/443 open (deploy is sudo, not root)
sudo fail2ban-client status sshd   # jail active
```

## Destroy

```bash
tofu destroy <same -var flags as apply>
```

Only removes the Tofu-managed firewall/attachment — the server itself was
created manually and must be deleted separately in the console.
