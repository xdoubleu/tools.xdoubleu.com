# infra

OpenTofu config for the Hetzner VPS (issue #1030) that hosts the self-hosted
stack (app + Postgres — see #1029, which replaced DO App Platform +
Supabase). Manages the firewall, OS-level hardening, and self-hosted Postgres
(issue #1031); the server itself is created manually. Self-hosted GoTrue
(issue #1032) used to be part of this stack too — issue #1039 replaced it
with a first-party implementation in `api` itself, so it's gone from here
entirely (see "GoTrue is gone" below).

**Tofu provisions the host; it does not deploy the app.** The app is
deployed by `.github/workflows/main.yml`'s `deploy-kamal` job on every push
to `main`, which is also where every app secret lives (as repo Secrets) —
see "Deploy the app via Kamal" below. Tofu used to run `kamal setup` itself
and carry a duplicate copy of all 25 app secrets as tfvars; that was removed
in #1113.

**Tofu applies automatically in CI, same as the app deploy** (issues
#1053/#1057/#1058/#1060). State lives in Cloudflare R2, not on any one
laptop — see "Remote state" below — and `.github/workflows/main.yml`'s
`infra-apply` job runs `tofu apply` on every push to `main` that touches
`infra/**`, with no manual approval step. Local `tofu plan`/`apply` still
work (see "Apply" below) for iterating on a change before opening a PR, or
as the manual escape hatch if CI is down — same relationship local Kamal
commands have to `deploy-kamal`.

## Remote state

State is stored in a dedicated Cloudflare R2 bucket (S3-compatible), not
the app's own `R2_BUCKET` — a separate bucket and a separate, narrowly
scoped API token, so a leaked credential for one doesn't imply access to
the other. `infra/versions.tf`'s `backend "s3"` block holds everything
that isn't account-specific (bucket name, `use_lockfile` for locking — R2
has no DynamoDB-equivalent, so this relies on OpenTofu 1.10+'s native
S3-backend lockfile locking — and the `skip_*` flags R2's API needs since
it isn't a full AWS S3 implementation). The endpoint URL embeds your
Cloudflare account ID, which isn't hardcoded into a committed file; it's
supplied at `tofu init` time via `-backend-config`.

**One-time setup:**

1. Cloudflare dashboard → R2 → create a bucket (e.g.
   `tools-xdoubleu-com-tfstate`), matching the `bucket` name in
   `infra/versions.tf`.
2. R2 → Manage API tokens → create a token scoped to **only** that bucket,
   Object Read & Write. Note the Access Key ID/Secret Access Key pair R2
   gives you (its S3-compatible credentials, not the Cloudflare API token
   itself) and your account's R2 endpoint
   (`https://<account-id>.r2.cloudflarestorage.com`).
3. `cp infra/backend.hcl.example infra/backend.hcl` (gitignored) and fill in
   the endpoint.
4. Migrate the existing local state up:
   ```bash
   cd infra
   export AWS_ACCESS_KEY_ID=<r2 access key id>
   export AWS_SECRET_ACCESS_KEY=<r2 secret access key>
   tofu init -backend-config=backend.hcl -migrate-state
   ```
   Confirm `tofu plan` shows no diff afterward — that proves the migrated
   state matches reality, not just that the migration command succeeded.

Every local `tofu` command from then on needs those same two env vars set
(and `-backend-config=backend.hcl` on `init`, once per checkout/`.terraform`
directory) — `tofu plan`/`apply` themselves take no extra flags for this.

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
Tofu auto-loads it, so `plan`/`apply` need no `-var` flags at all. `init`
needs the R2 backend credentials from "Remote state" above and
`-backend-config=backend.hcl` (once per checkout/`.terraform` directory):

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars   # fill in real values
export AWS_ACCESS_KEY_ID=<r2 access key id> AWS_SECRET_ACCESS_KEY=<r2 secret access key>
tofu init -backend-config=backend.hcl
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
  -var 'deploy_ssh_public_keys=["'"$(cat ~/.ssh/<key>.pub)"'", "'"$(cat ~/.ssh/kamal_ci_deploy.pub)"'"]'
tofu apply <same -var flags as plan>
```

This attaches the firewall (22/80/443 only) and runs `harden.sh` over SSH:
creates a non-root `deploy` user (passwordless sudo + docker groups, your
public key authorized), installs Docker, enables `fail2ban`, configures
`ufw`, configures `unattended-upgrades` for automatic security-only patches
with a scheduled 04:00 UTC reboot window if a kernel update needs one, and
disables root/password SSH login. It then stands up self-hosted
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
full reasoning — moot for GoTrue now that it's gone for good (issue #1039),
but still relevant if self-hosted Storage/Realtime are ever in scope.

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

## GoTrue is gone (issue #1039)

This section used to describe standing up a self-hosted `gotrue` service in
`postgres-compose.yml` (issue #1032). As of issue #1039, auth (password
sign-in, TOTP MFA, and the MCP OAuth 2.1 authorization server) is entirely
first-party — `api/internal/auth` and `api/internal/oauth2as` against
`api`'s own `auth` Postgres schema — and `api` never talks to a `gotrue`
container at all. The `gotrue` service has been removed from
`postgres-compose.yml`, and its `gotrue_*`/`resend_api_key` Tofu variables
are gone from `variables.tf`/`main.tf`.

The one-time cutover this used to require by hand (renaming the
Supabase-restored `auth` schema out of the way, running a standalone
migration script) is now **fully automatic**: `api/cmd/api/migrations/
00017_auth_schema.sql` detects a GoTrue-shaped `auth` schema (via the
presence of `auth.instances`, a table name only GoTrue/Supabase ever
creates) and renames it to `auth_gotrue_legacy` before creating the new
tables; `api/internal/legacyauth` then copies existing users' bcrypt
password hashes and verified TOTP factors across, idempotently, every time
`api` boots. A normal deploy of this change is the entire cutover — no
maintenance window, no manual `psql`/script step.

`auth_gotrue_legacy` itself is never dropped by any of this — it's left in
place in Postgres as a rollback fallback. If a rollback is ever needed:
redeploy the previous `api`/`web` images, then manually
`ALTER SCHEMA auth_gotrue_legacy RENAME TO auth;` (undoing that migration's
`DROP TABLE`s on the new schema first, via `goose down`, if it already ran).

## Deploy the app via Kamal (issue #1033)

**Deploys happen in CI, not here** — see the next section. Tofu stops at the
host; nothing under `infra/` runs Kamal. This section covers the one-time
bootstrap and the manual escape hatch.

Since cutover (#1034) the app is served on the real domain, not the raw IP:
`config/deploy.api.yml`/`config/deploy.web.yml` both set
`proxy.host: tools.xdoubleu.com` + `proxy.ssl: true`, so kamal-proxy obtains
and renews a Let's Encrypt cert itself over the HTTP-01 challenge (port 80,
already open in `hcloud_firewall.vps`) — nothing to configure per deploy.
`api` and `web` deploy as two independent Kamal services sharing that one
kamal-proxy instance and domain (issue #1038) — kamal-proxy routes
`/api/*`/`/.well-known/*` to `api` (`config/deploy.api.yml`'s
`proxy.path_prefix`) and everything else to `web`. Postgres stays exactly as
OpenTofu manages it above (**not** a Kamal accessory) — the app containers
Kamal starts reach it over the shared `kamal` Docker network
`null_resource.kamal_network` creates before Postgres comes up (see that
resource's comment in `infra/main.tf` for why the ordering matters).

### One-time bootstrap

The CI job runs `kamal deploy` for each of `api`/`web`, which assumes
kamal-proxy is already installed on the host. A fresh host needs
`kamal setup -c config/deploy.api.yml` **and**
`kamal setup -c config/deploy.web.yml` once, by hand — each deploys that one
service for the first time; kamal-proxy itself only actually installs on
whichever runs first (idempotent on the second). This has already been done
for the current VPS — you only need it if you rebuild the box.

### Manual deploy or rollback

Also how you'd deploy if CI is down. Needs Ruby 3.0+ and `bundle install`
(the repo's `Gemfile` pins the Kamal version CI uses — don't `gem install
kamal` separately and drift). **On macOS the system Ruby at `/usr/bin/ruby`
doesn't qualify** (stuck on 2.6, root-owned gem dir); `brew install ruby` and
put it ahead of the system one on `PATH`.

`config/deploy.api.yml`/`config/deploy.web.yml` are committed and read
as-is — Kamal evaluates each as ERB, so there is no render step; each just
needs the right environment and its own `-c` flag.

```bash
# 1. The two values config/deploy.api.yml/deploy.web.yml both read via ERB
export KAMAL_SERVER_IP=<vps ip> KAMAL_REGISTRY_USERNAME=<ghcr user>

# 2. Every name .kamal/secrets references — same values as the repo Secrets
#    in the CI job's env: block, which is where they actually live. Both
#    deploys read from the same .kamal/secrets file; each config's own
#    env.secret only pulls the names it references.
export RELEASE=<full sha> DB_DSN=... KAMAL_REGISTRY_PASSWORD=...   # etc.

# 3. Deploy specific already-built images (--skip-push: CI built and pushed
#    them; the tag is the full commit SHA, per build-api.yml/build-web.yml)
bundle exec kamal deploy -c config/deploy.api.yml --skip-push --version=<sha>
bundle exec kamal deploy -c config/deploy.web.yml --skip-push --version=<sha>
```

`bundle exec kamal config -c config/deploy.api.yml` (or `.web.yml`) renders
everything without deploying — use it to check the environment is complete
before a real run.

To roll back, pass an earlier `--version` (or `bundle exec kamal rollback -c
config/deploy.api.yml`/`.web.yml`) — each service rolls back independently,
without touching the other. Kamal will not cut traffic to a container that
fails its `/health` readiness probe — a bad deploy leaves the previous
container serving rather than taking the site down.

Verify with `curl https://tools.xdoubleu.com/api/version` (api) and
`curl https://tools.xdoubleu.com/` (web) plus a real sign-in through the app.

## Automate Kamal deploys in CI (issue #1036)

`.github/workflows/main.yml`'s `deploy-kamal` job is **the** deploy: it runs
on every push to `main`, and since cutover (#1034) DNS resolves to this VPS,
so a failure there means `main` didn't ship and fails the workflow — no
`continue-on-error`. The DigitalOcean App Platform `deploy` job that used to
run alongside it was removed in #1113, together with `do-app.yaml` and the
`DO_ACCESS_TOKEN`/`DO_APP_ID` secrets.

The repo Secrets listed below are the **single source of truth** for every
app secret — they are deliberately not duplicated as tfvars.
It runs `kamal deploy` (not `setup`) twice — once against
`config/deploy.api.yml`, once against `config/deploy.web.yml` (issue
#1038's two independent-service split) — against the already-bootstrapped
host, authenticating over SSH via a real `ssh-agent`
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
gh secret set KAMAL_SSH_KEY --repo <owner>/<repo> --env production < ~/.ssh/kamal_ci_deploy
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

**One-time setup**, GitHub repo Settings → Environments → `production` →
Environment secrets (not the repo-level Secrets tab — the `deploy-kamal`
job runs with `environment: production`, an Environment branch-restricted
to `main` with no required reviewers, so only a push to `main` can ever
populate its `secrets.*` context). All of these — including
`KAMAL_SERVER_IP` and `KAMAL_REGISTRY_USERNAME` — are Secrets, not
Variables: this repo is public, and GitHub only masks Secrets from
workflow logs, not Variables, and `KAMAL_SERVER_IP` in particular gets
echoed into a `ssh-keyscan` command, so a Variable would leak the VPS's IP
into a public log. Same
values as the matching `terraform.tfvars` entries below, under these exact
names (two are prefixed since GitHub Actions rejects secret names starting
with `GITHUB_`; the app-level env var Kamal actually sets on the container
is unaffected, only the GitHub-side secret name changes). `KAMAL_SERVER_IP`
and `KAMAL_REGISTRY_USERNAME` match `server_ip`/the GHCR user; the rest are
app secrets that exist **only** here — they are not tfvars:
```
KAMAL_SERVER_IP              (same value as server_ip in terraform.tfvars)
KAMAL_REGISTRY_USERNAME      (GHCR username; required by Kamal's config
                              schema even though the app image is a public
                              package needing no auth to pull)
KAMAL_SSH_KEY                (the dedicated CI deploy key's private half —
                              its public half is one entry in
                              deploy_ssh_public_keys in terraform.tfvars)
KAMAL_DB_DSN                 (postgres://postgres:<tofu output -raw
                              postgres_password>@postgres:5432/postgres —
                              Tofu generates the password but nothing outside
                              its state can read it, so copy it in once here;
                              rotating it means updating both)
KAMAL_REGISTRY_PASSWORD
JWT_SECRET                   (signs api's self-issued session JWTs, issue
                              #1039 — rotating it signs everyone out)
OAUTH_HMAC_SECRET            (keys the embedded MCP OAuth 2.1 authorization
                              server's token strategy, issue #1039 —
                              rotating it invalidates every issued MCP token)
STEAM_API_KEY
HARDCOVER_API_KEY
R2_ACCOUNT_ID
R2_ACCESS_KEY_ID
R2_SECRET_ACCESS_KEY
R2_BUCKET
SENTRY_DSN
SENTRY_DSN_WEB
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

## Automate infra apply in CI (issues #1053/#1057/#1058/#1060)

`.github/workflows/main.yml`'s `infra-apply` job runs on every push to
`main` that touches `infra/**` (`environment: production`, the same
branch-restricted Environment `deploy-kamal` uses — only a push to `main`
can populate its secrets). It:

1. Loads the same `KAMAL_SSH_KEY` deploy-kamal uses (already one of the
   authorized `deploy_ssh_public_keys` entries) and trusts the VPS host
   key, same steps as `deploy-kamal`.
2. `tofu init`s against the R2 backend.
3. **Snapshots the VPS** via the Hetzner API
   (`POST /servers/{id}/actions/create_image`) and polls until the snapshot
   is actually ready, labeled `purpose=ci-pre-apply` so later steps (and a
   human browsing the Hetzner console) can tell CI's snapshots apart from
   any you take by hand.
4. `tofu apply -auto-approve`.
5. **On failure**, calls the Hetzner API to rebuild the server from that
   snapshot (`POST /servers/{id}/actions/rebuild`) automatically, then lets
   the job stay failed. This is a real but lossy rollback: rebuilding from a
   snapshot causes a few minutes of downtime and discards anything written
   (including to Postgres) between the snapshot and the failure — accepted
   because there's no clean "fix forward" undo for a broken `harden.sh` or
   `postgres-compose.yml` mutation applied over SSH, unlike the app's own
   deploy (Kamal already won't cut traffic to a container that fails its
   health check).
6. **Always** prunes old CI snapshots, keeping only the newest 5.

There is deliberately **no manual approval gate** — not even scoped to the
Postgres-touching resources — and deliberately no PR-time `tofu plan`
check either: running `tofu plan` against the real remote state from a
`pull_request` trigger would need the same credentials as `apply`
(including `HCLOUD_TOKEN` and the R2 write key), but `production`'s branch
restriction — the whole point of putting these secrets there instead of
plain repo Secrets — means a PR run can't reach them without either
duplicating the credentials outside that Environment (undoing the
protection) or accepting that a PR could read them. The automatic
snapshot/restore above is the safety net instead.

**If the auto-restore step itself doesn't run** (e.g. the job was
cancelled mid-apply, before reaching that step): restore manually from the
Hetzner console (Servers → the VPS → Snapshots → find the newest
`ci-pre-apply`-labeled one → Rebuild from Image) or via the same API call
the workflow uses, with `$HCLOUD_TOKEN` and the image ID from either the
console or `curl https://api.hetzner.cloud/v1/images?type=snapshot&label_selector=purpose=ci-pre-apply`.

**One-time setup**, GitHub repo Settings → Environments → `production` →
Environment secrets, alongside `deploy-kamal`'s existing ones:
```
HCLOUD_TOKEN                   (Hetzner Cloud API token, read+write)
INFRA_SERVER_ID                (same value as server_id in terraform.tfvars)
TF_STATE_R2_ACCESS_KEY_ID       (the scoped R2 token's Access Key ID)
TF_STATE_R2_SECRET_ACCESS_KEY   (the scoped R2 token's Secret Access Key)
TF_STATE_R2_ENDPOINT            (same value as in infra/backend.hcl)
```
Plus two repo-level Variables (Settings → Secrets and variables → Actions
→ Variables tab — not Secrets, since neither value is sensitive: public
keys and the app's own public URL):
```
INFRA_DEPLOY_SSH_PUBLIC_KEYS   (JSON array of literal key text, e.g.
                                '["ssh-ed25519 AAAA... me", "ssh-ed25519 AAAA... kamal-ci-deploy"]' —
                                same keys as deploy_ssh_public_keys in
                                terraform.tfvars, but as literal text: the
                                CI runner has no access to your local
                                ~/.ssh/*.pub files to read a path from)
```
`KAMAL_SERVER_IP` is reused from `deploy-kamal`'s existing secrets — no new
value needed.

**Verify**: push a trivial change to `infra/harden.sh` (a comment edit is
enough), confirm `infra-apply` runs and succeeds in the Actions tab, and
that a new snapshot appears in the Hetzner console labeled
`ci-pre-apply`.

## Cutover (issue #1034)

Done — `tools.xdoubleu.com` resolves to the VPS and the app serves from it.
For the record, all it took was:

1. Point Cloudflare's A/AAAA records for the apex at the VPS's IP, leaving
   every other record (Resend's SPF/DKIM/DMARC in particular) untouched.
2. `proxy.host`/`proxy.ssl` in `config/deploy.yml` (now
   `config/deploy.api.yml`/`config/deploy.web.yml` post-#1038), plus
   `WEB_URL`/`API_URL` moved off the raw IP onto `https://tools.xdoubleu.com`
   — then the next deploy picks it up and kamal-proxy issues the cert on its
   next boot. Watch it happen with
   `ssh deploy@<ip> docker logs kamal-proxy -f`; a stuck challenge shows up
   there rather than in the app's own logs.

**DigitalOcean is decommissioned** (#1113): the `deploy` job, `do-app.yaml`,
and the `DO_ACCESS_TOKEN`/`DO_APP_ID` secrets are gone, and the App Platform
app itself can be deleted. Unrelated to `DO_OAUTH_CLIENT_ID`/`SECRET`, which
stay — those belong to the app's DigitalOcean monitoring *feature*
(`api/internal/digitalocean`), not to hosting.

**Update (issue #1039):** `api` no longer talks to Supabase or GoTrue at
all — password auth, TOTP MFA, and the MCP OAuth 2.1 authorization server
(`api/cmd/api/mcp.go`'s issuer) are now first-party, backed by `api`'s own
`auth` Postgres schema (`api/internal/auth`, `api/internal/oauth2as`). The
`SUPABASE_*`/`GOTRUE_URL` secrets have been removed from
`config/deploy.api.yml`, and the `gotrue` **container** itself has been
removed from `infra/` entirely — see "GoTrue is gone" above. The cutover
(renaming the legacy schema, copying existing users' password hashes/TOTP
factors) is automatic, run by `api` on every boot; there is no separate
follow-up step left to do.

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
systemctl is-active unattended-upgrades   # active
cat /etc/apt/apt.conf.d/20auto-upgrades   # both Periodic settings "1"
sudo unattended-upgrade --dry-run --debug # shows planned actions, no changes made

# Postgres: tunnel in (never exposed publicly) and check the migrated data
ssh -L 5432:localhost:5432 deploy@<ip>
# in another shell, using the password from `tofu output -raw postgres_password`:
psql "postgres://postgres:<password>@localhost:5432/postgres" -c '\dt auth.*'
psql "postgres://postgres:<password>@localhost:5432/postgres" -c '\dn'
# after the first api deploy post-#1039: auth_gotrue_legacy should now exist
# alongside a freshly populated auth schema — confirms the automatic cutover
# ran (see "GoTrue is gone" above).

# Auth (first-party as of #1039, no separate service to tunnel to — go
# through the app itself): sign in with an existing migrated account
curl -X POST https://tools.xdoubleu.com/api/auth.v1.AuthService/SignIn \
  -H 'Content-Type: application/json' \
  -d '{"email":"<existing-account-email>","password":"<...>"}'
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
