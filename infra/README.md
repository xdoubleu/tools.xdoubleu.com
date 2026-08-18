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
   → create. Note the server ID and public IPv4 shown after creation.
   Hetzner adds that key to root's `authorized_keys`, which is needed for
   exactly one thing: `null_resource.harden`'s very first run, which creates
   the `deploy` user and then permanently disables root SSH
   (`PermitRootLogin no`). Every run after that — including re-running
   `harden.sh` itself when it changes — connects as `deploy` (with
   passwordless sudo) instead, since root never works again past that
   first run. If you're bootstrapping a brand-new server and something
   goes wrong before that first run completes, SSH in as root manually and
   run `harden.sh` by hand to get `deploy` set up, then let Tofu take over.
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
  -var 'deploy_ssh_public_keys=["'"$(cat ~/.ssh/<key>.pub)"'", "'"$(cat ~/.ssh/kamal_ci_deploy.pub)"'"]' \
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

## Deploy the app via Kamal (issue #1033)

Fully Tofu-managed, same as everything else above: `tofu apply` is the one
command that provisions the VPS, stands up Postgres/GoTrue, **and** deploys
the app via Kamal — no separate manual `kamal setup`/`kamal deploy` step.
`config/deploy.yml` is generated (gitignored) from
`infra/templates/deploy.yml.tftpl` by `local_file.kamal_deploy_config`;
`infra/main.tf`'s `null_resource.kamal_deploy` shells out to `kamal setup`
locally via a `local-exec` provisioner. Since cutover (#1034) it deploys on
the real domain, not the raw IP: `deploy.yml.tftpl` sets `proxy.host:
tools.xdoubleu.com` + `proxy.ssl: true`, so kamal-proxy obtains and renews a
Let's Encrypt cert itself over the HTTP-01 challenge (port 80, already open
in `hcloud_firewall.vps`) — nothing to configure per deploy. Postgres and
GoTrue stay exactly as OpenTofu already manages them above (**not** Kamal
accessories) — the app container Kamal starts reaches them over the shared
`kamal` Docker network `null_resource.kamal_network` creates before Postgres
comes up (see that resource's comment in `infra/main.tf` for why the
ordering matters).

1. Make sure Ruby 3.0+ is on the machine you'll run `tofu apply` from
   (`local-exec` invokes the `kamal` binary there, not on the VPS or in
   CI) — that's the one thing Tofu can't install for you. The `kamal` gem
   itself doesn't need a separate install step: `null_resource.kamal_deploy`
   runs `gem install kamal --conservative` (no-op if already installed)
   before every `kamal setup`.
   **On macOS, the system Ruby at `/usr/bin/ruby` doesn't qualify** (stuck
   on 2.6, and its gem directory is root-owned, so `gem install` fails with
   a `Gem::FilePermissionError` even if the version were new enough).
   Install a real one instead — `brew install ruby`, then put it ahead of
   the system one on `PATH` (`echo 'export
   PATH="/opt/homebrew/opt/ruby/bin:$PATH"' >> ~/.zshrc && source
   ~/.zshrc` on Apple Silicon, `/usr/local/opt/ruby/bin` on Intel) — before
   running `tofu apply`.
2. Add every app secret `config/deploy.yml` references to your
   `terraform.tfvars`, alongside the vars from the sections above — same
   `sensitive` tfvar convention as `gotrue_jwt_secret`/`resend_api_key`, same
   values as today's DO App Platform deploy (`do-app.yaml`'s `SECRET` list),
   plus `kamal_registry_username`/`kamal_registry_password` (a GHCR PAT with
   `read:packages` scope — required by Kamal's own config schema even
   though `ghcr.io/xdoubleu/tools.xdoubleu.com/app` is a public package
   needing no auth to actually pull, confirmed via `kamal config`). See
   `infra/terraform.tfvars.example` for the full set of names.
   `RELEASE`/`DB_DSN`/`GOTRUE_URL` aren't tfvars — Tofu computes/injects
   those three itself (`null_resource.kamal_deploy` in `infra/main.tf`).
3. ```bash
   cd infra
   tofu apply   # same -var flags / terraform.tfvars as before
   ```
   This is idempotent: re-running with no new commit and unchanged
   secrets/`config/deploy.yml` skips the Kamal step entirely (its
   `triggers` didn't change); after a new commit whose image `docker.yml`'s
   CI already pushed to GHCR, `tofu apply` redeploys automatically. It always
   deploys the newest commit at-or-below `HEAD` that *has* an image
   (`infra/deployable-image.sh`), so an infra-only or docs-only `HEAD` — which
   `docker.yml` never builds — is a no-op rather than a deploy of a tag that
   doesn't exist.
4. Verify: `curl https://tools.xdoubleu.com/health`, sign in with a migrated account through
   the app itself (not just GoTrue directly — this is the first end-to-end
   test of `WithCustomAuthURL` repointing, not just #1032's isolated GoTrue
   smoke test).
5. **Mandatory rollback test**: `DB_DSN` is computed by Tofu, not a tfvar, so
   temporarily break it directly in `infra/main.tf`'s `null_resource.kamal_deploy`
   (point the `DB_DSN` line at an unreachable address instead of
   `random_password.postgres.result`), then:
   ```bash
   tofu apply -replace=null_resource.kamal_deploy
   ```
   Confirm Kamal refuses to cut traffic to the new, failing container:
   `/health` fails its readiness probe, the previous container keeps
   serving. Revert the `DB_DSN` line and re-apply (same `-replace`) to
   finish.

## Automate Kamal deploys in CI (issue #1036)

Once the steps above have bootstrapped the host at least once (kamal-proxy
installed, first deploy done), `.github/workflows/main.yml`'s
`deploy-kamal` job takes over routine deploys on every push to `main`.
Since cutover (#1034) it's the production deploy, so a failure there fails
the workflow — no `continue-on-error`. The old `deploy` job (DO App
Platform) still runs alongside it but is the one marked
`continue-on-error: true` now: DNS no longer points at DO, and it's kept
only so that component stays on a current image as a warm rollback target.
It runs `kamal deploy` (not `setup`) against the
already-bootstrapped host, authenticating over SSH via a real `ssh-agent`
(started by the job's own "Load the deploy SSH key" step, loading
`KAMAL_SSH_KEY`) — same auth mechanism as the local `tofu apply` path, just
with the key coming from a repo secret instead of whatever's already loaded
in your own agent.

That agent runs headless in CI and can't unlock a
passphrase-protected key, so don't reuse your own key here — generate a
dedicated, unencrypted CI deploy key, add its public half to
`deploy_ssh_public_keys` in `terraform.tfvars` (alongside your own key) and
re-`tofu apply` so `harden.sh` authorizes it on the VPS, then store the
private half as `KAMAL_SSH_KEY` below:
```bash
ssh-keygen -t ed25519 -f ~/.ssh/kamal_ci_deploy -N "" -C "kamal-ci-deploy"
gh secret set KAMAL_SSH_KEY --repo <owner>/<repo> < ~/.ssh/kamal_ci_deploy
```

Set it from the file like that rather than pasting into the web UI — the UI
strips the key's trailing newline, and OpenSSH rejects the resulting PEM with
`Error loading key "(stdin)": error in libcrypto` (issue #1106). The workflow
now re-adds the newline defensively, but a key pasted with other damage
(passphrase-protected, the `.pub` half, a PuTTY `.ppk`) still fails — the
`deploy-kamal` job says so explicitly when it does.

In `terraform.tfvars`, write each entry as either a **path** to the `.pub`
file (`"~/.ssh/kamal_ci_deploy.pub"` — `main.tf`'s `local.deploy_ssh_public_keys`
reads it) or the key's literal text. What does *not* work is
`"$(cat ~/.ssh/kamal_ci_deploy.pub)"`: unlike the
`-var 'deploy_ssh_public_keys=["'"$(cat ...)"'"]'` form above (where your shell
expands it before Tofu ever sees it), a `.tfvars` file is not shell-interpolated,
so that entry would be appended to the VPS's `authorized_keys` as that exact
string — sshd then ignores the unparsable line and the key never works.
`variables.tf` has a `validation` block rejecting it at plan time.

**One-time setup**, GitHub repo Settings → Secrets and variables → Actions,
Secrets tab. All of these — including `KAMAL_SERVER_IP` and
`KAMAL_REGISTRY_USERNAME` — are Secrets, not Variables: this repo is
public, and GitHub only masks Secrets from workflow logs, not Variables,
and `KAMAL_SERVER_IP` in particular gets echoed into a `ssh-keyscan`
command, so a Variable would leak the VPS's IP into a public log. Same
values as the matching `terraform.tfvars` entries below, under these exact
names (two are prefixed since GitHub Actions rejects secret names starting
with `GITHUB_`; the app-level env var Kamal actually sets on the container
is unaffected, only the GitHub-side secret name changes):
```
KAMAL_SERVER_IP              (same value as server_ip in terraform.tfvars)
KAMAL_REGISTRY_USERNAME      (same value as kamal_registry_username)
KAMAL_SSH_KEY                (the dedicated CI deploy key's private half —
                              its public half is one entry in
                              deploy_ssh_public_keys in terraform.tfvars)
KAMAL_DB_DSN                 (same DB_DSN null_resource.kamal_deploy
                              computes locally — postgres://postgres:<tofu
                              output -raw postgres_password>@postgres:5432/postgres)
KAMAL_REGISTRY_PASSWORD
SUPABASE_PROJ_REF
SUPABASE_API_KEY
STEAM_API_KEY
HARDCOVER_API_KEY
R2_ACCOUNT_ID
R2_ACCESS_KEY_ID
R2_SECRET_ACCESS_KEY
R2_BUCKET
SENTRY_DSN
SENTRY_DSN_WEB
SUPABASE_URL
SUPABASE_ANON_KEY
KAMAL_GITHUB_OAUTH_CLIENT_ID       (→ GITHUB_OAUTH_CLIENT_ID on the container)
KAMAL_GITHUB_OAUTH_CLIENT_SECRET   (→ GITHUB_OAUTH_CLIENT_SECRET)
SENTRY_OAUTH_CLIENT_ID
SENTRY_OAUTH_CLIENT_SECRET
DO_OAUTH_CLIENT_ID
DO_OAUTH_CLIENT_SECRET
ENCRYPTION_KEY
RESEND_API_KEY
EMAIL_FROM
NOTIFY_EMAIL_TO
EMAIL_INBOUND_DOMAIN
EMAIL_INBOUND_SECRET
```

**Verify**: push a trivial change to `main`, confirm `deploy-kamal` runs and
succeeds in the Actions tab, then `curl https://tools.xdoubleu.com/health`.

**External uptime monitoring** (also part of #1036, not automatable from
here — a manual account-setup step): an UptimeRobot free-tier monitor, 5
minute interval, against `https://tools.xdoubleu.com/health`.

## Cutover (issue #1034)

Done — `tools.xdoubleu.com` resolves to the VPS and the app serves from it.
For the record, all it took was:

1. Point Cloudflare's A/AAAA records for the apex at the VPS's IP, leaving
   every other record (Resend's SPF/DKIM/DMARC in particular) untouched.
2. `proxy.host`/`proxy.ssl` in `infra/templates/deploy.yml.tftpl`, plus
   `WEB_URL`/`API_URL` moved off the raw IP onto `https://tools.xdoubleu.com`
   — then any deploy (a push to `main`, or `tofu apply`) picks it up and
   kamal-proxy issues the cert on its next boot. Watch it happen with
   `ssh deploy@<ip> docker logs kamal-proxy -f`; a stuck challenge shows up
   there rather than in the app's own logs.
3. Set `gotrue_site_url` in `terraform.tfvars` to `https://tools.xdoubleu.com`
   if it wasn't already, so GoTrue's auth emails link at the real domain, and
   re-`tofu apply`.

The DO App Platform component and the Supabase project are left dormant, not
deleted — see the `deploy` job's comment in `.github/workflows/main.yml`.

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
