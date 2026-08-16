# infra

OpenTofu config for the Hetzner VPS (issue #1030) that will host the
self-hosted stack (app + Postgres + GoTrue, replacing DO App Platform +
Supabase — see #1029). Manages the firewall, OS-level hardening, and
self-hosted Postgres (issue #1031); the server itself is created manually.

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
`ufw`, and disables root/password SSH login. It then stands up self-hosted
Postgres — see below.

`harden.sh` is idempotent — re-running `tofu apply` after editing it (or with
no changes at all) is safe.

## Stand up Postgres

The same `tofu apply` above also creates `null_resource.postgres`, which
uploads `postgres-compose.yml` and a generated `.env` (a Tofu-managed
`random_password`) to `/home/deploy/postgres/` and runs
`docker compose up -d` as the `deploy` user (issue #1031). Before the first
apply, confirm the image tag pinned in `postgres-compose.yml` matches the
source Supabase project's actual Postgres version (Dashboard → Database).

Postgres is **not** exposed publicly — it's bound to `127.0.0.1:5432` on the
VPS only. Retrieve the generated password with:

```bash
tofu output -raw postgres_password
```

Re-running `apply` after editing `postgres-compose.yml` redeploys it;
rotating the password (`tofu apply -replace=random_password.postgres <same
-var flags>`) also forces a redeploy so the running container picks it up.

## Migrate data from Supabase (one-time)

Run once, after `tofu apply` has stood up Postgres, against an existing
`pg_dump --format=custom` of the source database (see `infra/migrate-db.sh`):

```bash
ssh deploy@<ip> docker ps   # confirm the postgres container's name

export DEPLOY_HOST=deploy@<ip>
export PG_CONTAINER=<name from docker ps>
./migrate-db.sh <path-to-dump-file>
```

This streams the dump file (which should include Supabase's `auth` schema,
not just the app schemas) straight into `pg_restore` on the VPS via SSH —
the file never touches disk on the VPS. Supabase's own database is
untouched throughout, so this is safe to re-run.

## Verify

```bash
ssh deploy@<ip>                    # should work, key auth only
ssh root@<ip>                      # should be rejected
sudo ufw status                    # on the box: only 22/80/443 open (deploy is sudo, not root)
sudo fail2ban-client status sshd   # jail active

# Postgres: tunnel in (never exposed publicly) and check the migrated data
ssh -L 5432:localhost:5432 deploy@<ip>
# in another shell, using the password from `tofu output -raw postgres_password`:
psql "postgres://postgres:<password>@localhost:5432/postgres" -c '\dt auth.*'
psql "postgres://postgres:<password>@localhost:5432/postgres" -c 'select * from goose_db_version'
```

## Destroy

```bash
tofu destroy <same -var flags as apply>
```

Only removes the Tofu-managed firewall/attachment — `null_resource.harden`
and `null_resource.postgres` have no destroy-time provisioner, so the
running container, its data volume, and the OS-level hardening are left in
place on the box. The server itself was created manually and must be
deleted separately in the console.
