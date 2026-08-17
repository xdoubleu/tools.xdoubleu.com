# infra

OpenTofu config for the Hetzner VPS (issue #1030) that will host the
self-hosted stack (app + Postgres + GoTrue, replacing DO App Platform +
Supabase — see #1029). Manages the firewall, OS-level hardening, and
self-hosted Postgres (issue #1031) and GoTrue (issue #1032); the server
itself is created manually.

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

Tofu doesn't persist `-var` values between runs, so passing them on every
`plan`/`apply` gets old fast. Copy `terraform.tfvars.example` to
`terraform.tfvars` (gitignored — never commit it) and fill in real values;
Tofu auto-loads it, so `plan`/`apply` need no `-var` flags at all:

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars   # fill in real values
tofu init
tofu plan
tofu apply
```

Or, without a `terraform.tfvars` file, pass everything explicitly each time:

```bash
cd infra
export HCLOUD_TOKEN=<your token>
tofu plan \
  -var hcloud_token="$HCLOUD_TOKEN" \
  -var server_id=<id> \
  -var server_ip=<ip> \
  -var deploy_ssh_public_key="$(cat ~/.ssh/<key>.pub)" \
  -var gotrue_jwt_secret=<...> \
  -var resend_api_key=<...> \
  -var gotrue_site_url=<...> \
  -var gotrue_smtp_admin_email=<...>
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
`docker compose up -d` as the `deploy` user (issue #1031). Runs plain
`postgres:17` — not the `supabase/postgres` image — since the latter's
built-in `auth`/`storage`/`realtime` schemas are a generic starter
baseline that doesn't match a real project's actual migration history,
and its `postgres` role isn't a true superuser, which blocks fixing that
mismatch by hand. See the image comment in `postgres-compose.yml` for the
full reasoning; this only matters again once self-hosted GoTrue/Storage
are actually in scope (a separate future sub-issue).

Postgres is **not** exposed publicly — it's bound to `127.0.0.1:5432` on the
VPS only. Retrieve the generated password with:

```bash
tofu output -raw postgres_password
```

Re-running `apply` after editing `postgres-compose.yml` redeploys it;
rotating the password (`tofu apply -replace=random_password.postgres <same
-var flags>`) also forces a redeploy so the running container picks it up.

**Changing the image on an already-running instance** doesn't reset an
existing data volume. If you need a clean slate (e.g. retrying a
migration), wipe the volume and let it start fresh:

```bash
ssh deploy@<ip> "cd postgres && docker compose down -v && docker compose up -d"
```

## Stand up GoTrue

The same `tofu apply` also creates a `gotrue` service in
`postgres-compose.yml` (issue #1032), pointed at the restored `auth` schema
above via `GOTRUE_DB_DATABASE_URL` (compose network hostname `postgres`, not
`127.0.0.1`). It runs `supabase/gotrue:v2.195.0` — pinned, not `latest`, for
reproducibility — and lets GoTrue apply its own internal migrations against
that schema on first boot; watch `docker logs` for migration errors when
first applying.

Pass four extra `-var` flags alongside the ones above:

- `gotrue_jwt_secret` — pull the **actual** value from the Supabase
  dashboard (Project Settings → API), not a freshly generated one, so
  already-issued client JWTs keep validating post-cutover instead of forcing
  every signed-in user to re-authenticate.
- `resend_api_key` — same Resend account already used by
  `api/internal/mailer`; used as the password for GoTrue's SMTP relay
  (`smtp.resend.com:465`).
- `gotrue_site_url` — the app's public URL.
- `gotrue_smtp_admin_email` — the From address for GoTrue's own auth emails.

Like Postgres, GoTrue is bound to `127.0.0.1:9999` — not exposed publicly.

Before flipping DNS over to self-hosted GoTrue (not part of this smoke test):
check the Supabase dashboard's Auth → Providers page for any enabled
third-party providers (Google, GitHub, etc.) beyond email/password — their
redirect URIs need updating at the provider to point at self-hosted GoTrue's
callback endpoint first, or that sign-in method breaks silently. Also note
(don't patch) whether self-hosted OSS GoTrue lacks the OAuth-server/dynamic-
client-registration feature the MCP flow (`api/cmd/api/mcp.go`) relies on —
closed properly by the future "retire GoTrue entirely" work instead.

## Migrate data from Supabase (one-time)

Run once, after `tofu apply` has stood up Postgres, against an existing
plain-SQL `pg_dump` of the source database (including Supabase's `auth`
schema, not just the app schemas). Streams straight into `psql` on the VPS
via SSH — the file never touches disk on the VPS:

```bash
ssh deploy@<ip> docker ps   # confirm the postgres container's name

ssh deploy@<ip> "docker exec -i <container> psql --username=postgres --dbname=postgres" \
  <path-to-dump-file>
```

If the dump was produced by a newer `pg_dump` (PostgreSQL 17+), it may be
wrapped in `\restrict <token>` / `\unrestrict <token>` lines — a safety
feature that, once triggered, blocks every other backslash command in the
file (including version-conditional `\if`/`\endif` guards pg_dump itself
inserts), silently skipping whatever they guard. Strip both lines before
restoring if so:

```bash
sed -i '' '/^\\restrict /d; /^\\unrestrict /d' <path-to-dump-file>
```

Supabase's own database is untouched throughout, so this is safe to re-run.

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

# GoTrue: tunnel in and sign in with an existing migrated account
ssh -L 9999:localhost:9999 deploy@<ip>
# in another shell:
curl -X POST 'http://localhost:9999/token?grant_type=password' \
  -H 'Content-Type: application/json' \
  -d '{"email":"<existing-account-email>","password":"<...>"}'
# then, with the access_token from the response:
curl http://localhost:9999/user -H 'Authorization: Bearer <access_token>'
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
