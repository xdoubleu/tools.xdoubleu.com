# infra

OpenTofu config for the Hetzner VPS that hosts the self-hosted stack: the
firewall, OS-level hardening, and self-hosted Postgres plus the metrics
accessories. The server itself is created manually.

- **Tofu provisions the host; it does not deploy the app.** The app is
  deployed by `.github/workflows/main.yml`'s `deploy-kamal` job on every push
  to `main`, and every app secret lives there as an Environment secret — see
  "Deploy the app via Kamal". Secrets are never duplicated as tfvars.
- **Tofu applies automatically in CI.** State lives in Cloudflare R2 ("Remote
  state"); the `infra-apply` job runs `tofu apply` on every push to `main`
  that touches `infra/**`, with no manual approval. Local `tofu plan`/`apply`
  ("Apply") are for iterating before a PR, or the escape hatch if CI is down.

## Remote state

State lives in a dedicated R2 bucket with its own narrowly scoped token —
separate from the app's `R2_BUCKET`, so one leaked credential doesn't imply the
other. `infra/versions.tf`'s `backend "s3"` block holds the bucket name,
`use_lockfile` (OpenTofu 1.10+ native locking; R2 has no DynamoDB equivalent),
and the `skip_*` flags R2 needs. The endpoint embeds the Cloudflare account ID,
so it's supplied at `tofu init` via `-backend-config`.

**One-time setup:**

1. Cloudflare → R2 → create a bucket (e.g. `tools-xdoubleu-com-tfstate`)
   matching `bucket` in `infra/versions.tf`.
2. R2 → Manage API tokens → token scoped to **only** that bucket, Object Read &
   Write. Note its S3-compatible Access Key ID / Secret Access Key and the
   endpoint (`https://<account-id>.r2.cloudflarestorage.com`).
3. `cp infra/backend.hcl.example infra/backend.hcl` (gitignored) and fill in
   the endpoint.
4. Migrate existing local state:
   ```bash
   cd infra
   export AWS_ACCESS_KEY_ID=<r2 access key id>
   export AWS_SECRET_ACCESS_KEY=<r2 secret access key>
   tofu init -backend-config=backend.hcl -migrate-state
   ```
   Confirm `tofu plan` shows no diff afterward.

Every local `tofu` command needs those two env vars (and
`-backend-config=backend.hcl` on `init`, once per `.terraform` directory).

## One-time setup

1. **Create the server** in the [Hetzner Console](https://console.hetzner.cloud/):
   location `Falkenstein (fsn1)`, image `Ubuntu 26.04`, type `CX23` (or `CX22`,
   same tier), your SSH public key under "SSH keys". Note the server ID and
   public IPv4. Root's key is used only by `null_resource.harden`'s first run,
   which creates the `deploy` user and disables root SSH (`PermitRootLogin no`);
   every later run connects as `deploy`. If bootstrapping fails before that
   first run completes, SSH in as root and run `harden.sh` by hand.
2. **API token**: Hetzner Console → Security → API Tokens → generate (read+write).
3. Install OpenTofu (`brew install opentofu` or
   [opentofu.org](https://opentofu.org/docs/intro/install/)).
4. **Load your SSH key into `ssh-agent`** (`ssh-add --apple-use-keychain
   ~/.ssh/<key>`) — the provisioner can't read a passphrase-protected key file.

## Apply

Copy `terraform.tfvars.example` to `terraform.tfvars` (gitignored — never
commit it); Tofu auto-loads it, so `plan`/`apply` need no `-var` flags:

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars   # fill in real values
export AWS_ACCESS_KEY_ID=<r2 access key id> AWS_SECRET_ACCESS_KEY=<r2 secret access key>
tofu init -backend-config=backend.hcl
tofu plan
tofu apply
```

Or pass everything explicitly:

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
non-root `deploy` user (passwordless sudo + docker group), Docker, `fail2ban`,
`ufw`, `unattended-upgrades` (security-only, 04:00 UTC reboot window when a
kernel update needs it), and root/password SSH disabled. Then it stands up the
accessories below. `harden.sh` is idempotent — re-applying is always safe.

**Getting notified of a new Ubuntu LTS release:** `unattended-upgrades` never
runs `do-release-upgrade` (too risky unattended on a single box). Instead
`release-upgrade-check.sh` + the `release-upgrade-check.timer`/`.service`
systemd units (installed by `harden.sh`, configured by
`null_resource.release_upgrade_check`) run weekly **on the VPS**, calling
`do-release-upgrade -c`, and email `release_check_email_to` via Resend using
`release_check_resend_api_key`/`release_check_email_from`. In CI those come
from the `RESEND_API_KEY`/`EMAIL_FROM`/`NOTIFY_EMAIL_TO` secrets as `TF_VAR_*`;
they're written to a file on the VPS, not passed to a container. See
[ADR-0012](../docs/adr-0012-ubuntu-release-check-on-vps.md).

```bash
systemctl list-timers release-upgrade-check.timer   # confirm scheduled
sudo systemctl start release-upgrade-check.service  # trigger manually
```

## Stand up Postgres

`null_resource.postgres` uploads `postgres-compose.yml` and a generated `.env`
(Tofu-managed `random_password`) to `/home/deploy/postgres/` and runs
`docker compose up -d` as `deploy`. It runs plain `postgres:17`, not
`supabase/postgres` — see the image comment in `postgres-compose.yml`.

Postgres is bound to `127.0.0.1:5432` only, never public. Password:

```bash
tofu output -raw postgres_password
```

Re-applying after editing `postgres-compose.yml` redeploys it; rotating the
password (`tofu apply -replace=random_password.postgres <same -var flags>`)
also forces a redeploy.

Changing the image doesn't reset an existing data volume. For a clean slate:

```bash
ssh deploy@<ip> "cd postgres && docker compose down -v && docker compose up -d"
```

## Stand up node_exporter

`null_resource.node_exporter` uploads `node-exporter-compose.yml` to
`/home/deploy/node-exporter/` and runs `docker compose up -d`. No published
port and no secrets — reachable only from containers on the `kamal` network.
Re-applying after editing the compose file redeploys it. Host metrics are read
via Grafana (`/grafana`) or the `prom_query` MCP tool.

## Stand up Prometheus + postgres_exporter + Grafana

`null_resource.prometheus` uploads `prometheus-compose.yml`, `prometheus.yml`,
and a generated `.env` (postgres_exporter's `DATA_SOURCE_NAME`) to
`/home/deploy/prometheus/` and runs `docker compose up -d`. Prometheus only
collects; **alerting is Grafana's** (`infra/grafana/provisioning/alerting/`).
Neither Prometheus nor postgres_exporter publishes a host port.

It also uploads a `web_ingest_secret` file (`OBSERVABILITY_INGEST_SECRET`, via
`TF_VAR_observability_ingest_secret`) that `prometheus.yml`'s `web` job sends
as a bearer token, since `web`'s `GET /metrics` requires it. `deploy-kamal`
`needs` `infra-apply`, so Prometheus sends the token before `web` requires it.

Grafana is **not** Tofu-managed: it's a third Kamal service
(`config/deploy.grafana.yml`, deployed by `deploy-kamal`) because it needs the
public `/grafana` path through kamal-proxy. Its wrapper image
(`infra/grafana.Dockerfile`, built by `build-grafana.yml`) bakes in the
datasources, plugins, dashboards, and alerting from `infra/grafana/`.
Dashboard JSON under `infra/grafana/dashboards/` is the source of truth
(`allowUiUpdates: false`) — edit it and redeploy. `make lint/grafana` and
`make grafana/verify` check it (also run by `build-grafana.yml`). Rationale:
[ADR-0022](../docs/adr-0022-prometheus-grafana-metrics.md).

Re-applying redeploys the Prometheus accessory; `kamal deploy -c
config/deploy.grafana.yml` (or a push to `main` touching `infra/grafana/**` or
`infra/grafana.Dockerfile`) redeploys Grafana.

**Troubleshooting:** if `prom_query` reports a target `down`, check
`ssh deploy@<ip> docker ps` for `prometheus`/`postgres-exporter`/`node-exporter`.
If the `api` or `web` target is **missing entirely**, Docker service discovery
is broken. It needs: the `service` label on the container
(`docker inspect -f '{{.Config.Labels.service}}' <container>`), attachment to
the `kamal` network, and Docker-socket access — the image runs as `nobody`, so
`null_resource.prometheus` passes the host's docker gid via `DOCKER_GID` into
`group_add`. `docker logs prometheus | grep docker_sd` shows a permission
error plainly. The `TargetMissing` alert fires on this case.

## GoTrue is gone

Auth (password sign-in, TOTP MFA, the MCP OAuth 2.1 authorization server) is
first-party in `api` (`api/internal/auth`, `api/internal/oauth2as`) against its
own `auth` schema; there is no `gotrue` container. The one-time cutover was
automatic (`api/cmd/api/migrations/00017_auth_schema.sql`), and the
`auth_gotrue_legacy` fallback schema has since been dropped by
`00019_drop_auth_gotrue_legacy.sql`. See
[ADR-0005](../docs/adr-0005-first-party-auth-replacing-gotrue.md).

## Deploy the app via Kamal

**Deploys happen in CI** (next section); nothing under `infra/` runs Kamal.
This section is the one-time bootstrap and the manual escape hatch.

`config/deploy.api.yml`/`config/deploy.web.yml` set
`proxy.host: tools.xdoubleu.com` + `proxy.ssl: true`, so kamal-proxy
obtains and renews a Let's Encrypt cert over HTTP-01 (port 80). `api` and `web`
are two independent Kamal services sharing one kamal-proxy: `/api/*` and
`/.well-known/*` go to `api` (`proxy.path_prefix`), everything else to `web`.
Postgres stays Tofu-managed (not a Kamal accessory); app containers reach it
over the `kamal` Docker network `null_resource.kamal_network` creates first
(see its comment in `infra/main.tf`).

### One-time bootstrap

`deploy-kamal` runs `kamal deploy`, which assumes kamal-proxy is installed. A
fresh host needs, once, by hand:

```bash
bundle exec kamal setup -c config/deploy.api.yml
bundle exec kamal setup -c config/deploy.web.yml
```

Already done for the current VPS — only needed if the box is rebuilt.

### Manual deploy or rollback

Needs Ruby 3.0+ and `bundle install` (the `Gemfile` pins CI's Kamal version —
don't `gem install kamal`). On macOS, system Ruby (2.6) doesn't qualify;
`brew install ruby` and put it first on `PATH`. The configs are ERB, read as-is
with their own `-c` flag.

```bash
# 1. Values both configs read via ERB
export KAMAL_SERVER_IP=<vps ip> KAMAL_REGISTRY_USERNAME=<ghcr user>

# 2. Every name .kamal/secrets references — same values as the Environment
#    secrets below
export RELEASE=<full sha> DB_DSN=... KAMAL_REGISTRY_PASSWORD=...   # etc.

# 3. Deploy already-built images (tag = full commit SHA)
bundle exec kamal deploy -c config/deploy.api.yml --skip-push --version=<sha>
bundle exec kamal deploy -c config/deploy.web.yml --skip-push --version=<sha>
```

`bundle exec kamal config -c config/deploy.api.yml` (or `.web.yml`) renders the
config without deploying — use it to check the environment is complete.

Roll back with an earlier `--version` or `bundle exec kamal rollback -c
config/deploy.api.yml`/`.web.yml`; each service rolls back independently.
Kamal won't cut traffic to a container failing its `/health` probe, so a bad
deploy leaves the previous one serving.

Verify with `curl https://tools.xdoubleu.com/api/version`,
`curl https://tools.xdoubleu.com/`, and a real sign-in.

## Automate Kamal deploys in CI

`main.yml`'s `deploy-kamal` job is **the** deploy: every push to `main`, no
`continue-on-error`. It runs `kamal deploy` (not `setup`) against
`config/deploy.api.yml` and `config/deploy.web.yml` (and Grafana), over SSH via
an `ssh-agent` loaded with `KAMAL_SSH_KEY`.

**CI deploy key.** The CI agent is headless and can't unlock a passphrase, so
use a dedicated unencrypted key. Add its public half to
`deploy_ssh_public_keys` in `terraform.tfvars`, re-`tofu apply`, then store the
private half:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/kamal_ci_deploy -N "" -C "kamal-ci-deploy"
gh secret set KAMAL_SSH_KEY --repo <owner>/<repo> --env production < ~/.ssh/kamal_ci_deploy
```

Set it from the file, not the web UI — the UI strips the trailing newline
(`Error loading key "(stdin)": error in libcrypto`). The workflow re-adds it,
but a passphrase-protected key, the `.pub` half, or a `.ppk` still fails, and
`deploy-kamal` says so.

In `terraform.tfvars`, each `deploy_ssh_public_keys` entry is a **path** to the
`.pub` file (`"~/.ssh/kamal_ci_deploy.pub"`) or the key's literal text — never
`"$(cat ...)"`: tfvars aren't shell-interpolated, so that string would land in
`authorized_keys` verbatim. A `validation` block in `variables.tf` rejects it.

**One-time setup:** GitHub Settings → Environments → `production` →
Environment secrets (not repo-level Secrets). `deploy-kamal` runs with
`environment: production`, branch-restricted to `main`, so only a push to
`main` can read them. All of these are Secrets, not Variables — the repo is
public and only Secrets are masked in logs (`KAMAL_SERVER_IP` is echoed into
`ssh-keyscan`). Names prefixed `KAMAL_GITHUB_*`/`GRAFANA_GITHUB_*` exist
because GitHub rejects secret names starting with `GITHUB_`; the container env
var keeps its unprefixed name. App secrets exist **only** here:

```
KAMAL_SERVER_IP              (same value as server_ip in terraform.tfvars)
KAMAL_REGISTRY_USERNAME      (GHCR username; required by Kamal's schema even
                              though the images are public)
KAMAL_SSH_KEY                (CI deploy key's private half; public half is in
                              deploy_ssh_public_keys)
KAMAL_DB_DSN                 (postgres://postgres:<tofu output -raw
                              postgres_password>@postgres:5432/postgres —
                              copy it in once; rotating means updating both)
KAMAL_REGISTRY_PASSWORD      (also pulls the Grafana wrapper image from GHCR)
JWT_SECRET                   (signs api session JWTs — rotating signs everyone out)
OAUTH_HMAC_SECRET            (MCP OAuth AS token strategy — rotating invalidates
                              every issued MCP token)
OAUTH_OIDC_PRIVATE_KEY       (PEM RSA key signing OIDC ID tokens, public half at
                              /oauth2/jwks; unset ⇒ ephemeral per boot.
                              openssl genpkey -algorithm RSA
                              -pkeyopt rsa_keygen_bits:2048)
OAUTH_GRAFANA_CLIENT_SECRET  (static "grafana" OAuth client secret; api
                              reconciles its bcrypt hash on boot; unset ⇒
                              Grafana SSO unusable. Also passed to Grafana as
                              GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET)
GRAFANA_ADMIN_PASSWORD       (Grafana break-glass admin, GF_SECURITY_ADMIN_PASSWORD;
                              also given to api for get_grafana_alerts)
STEAM_API_KEY
HARDCOVER_API_KEY
BMC_PARTNER_KEY              (SNCB GTFS feed subscription key, trains app;
                              sent as the bmc-partner-key header)
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
TODOIST_OAUTH_CLIENT_ID      (learningpaths' Todoist connect flow; the app's own
                              client id — per-user tokens live in the DB)
TODOIST_OAUTH_CLIENT_SECRET
ENCRYPTION_KEY
RESEND_API_KEY               (also Grafana's SMTP password, GF_SMTP_PASSWORD)
EMAIL_FROM                   (also Grafana's GF_SMTP_FROM_ADDRESS)
NOTIFY_EMAIL_TO              (admin recipient for api notification emails)
EMAIL_INBOUND_DOMAIN
EMAIL_INBOUND_SECRET
OBSERVABILITY_INGEST_SECRET  (gates POST /api/observability/logs and web's
                              GET /metrics; also TF_VAR_observability_ingest_secret
                              for Prometheus's web scrape job)
GRAFANA_GITHUB_DATASOURCE_TOKEN  (fine-grained PAT scoped to this repo, for
                              grafana-github-datasource; read via $__env{} in
                              infra/grafana/provisioning/datasources/issue-signals.yml)
GRAFANA_SENTRY_DATASOURCE_TOKEN  (Sentry token, org:read + project:read +
                              event:read, for grafana-sentry-datasource)
GRAFANA_SLACK_WEBHOOK_URL    (Slack webhook for the alert contact point,
                              infra/grafana/provisioning/alerting/contactpoints.yml)
ROUTINE_FIRE_TOKEN           (bearer token for internal/routines.Client's POST to
                              ROUTINE_FIRE_URL and the inbound
                              POST /webhooks/grafana-alert; `openssl rand -hex 32`.
                              Unset ⇒ inbound webhook rejects everything. Also a
                              plain Actions secret for main.yml's
                              notify-main-ci-red job)
SLACK_WEBHOOK_URL            (Slack webhook the notify_slack MCP tool posts to;
                              separate from GRAFANA_SLACK_WEBHOOK_URL. Unset ⇒
                              ErrNotConfigured)
POSTHOG_KEY                  (web's PostHog Cloud EU project key; POSTHOG_HOST
                              is plain env.clear in config/deploy.web.yml)
```

Every name in a deploy config's `env.secret:` must also be in `.kamal/secrets`
**and** the matching `Deploy <svc> via Kamal` step's `env:` in `main.yml`, or
`kamal deploy` aborts post-merge. `make lint/kamal-secrets` catches a
mismatch; a new secret still needs adding to all three plus the Environment
secret above → [convention-deploy-secrets](../docs/convention-deploy-secrets.md).

**Verify**: push a trivial change to `main`, confirm `deploy-kamal` succeeds,
then `curl https://tools.xdoubleu.com/health`.

**External uptime monitoring** (manual account setup): an UptimeRobot
free-tier monitor, 5-minute interval, on `https://tools.xdoubleu.com/health`.

## Automate infra apply in CI

`main.yml`'s `infra-apply` job runs on every push to `main` touching
`infra/**` (`environment: production`). It:

1. Loads `KAMAL_SSH_KEY` and trusts the VPS host key.
2. `tofu init`s against the R2 backend.
3. **Snapshots the VPS** (`POST /servers/{id}/actions/create_image`), labeled
   `purpose=ci-pre-apply`, polling up to 30 min until ready; the image id is
   published only once ready.
4. `tofu apply -auto-approve`.
5. **On failure**, rebuilds the server from that snapshot
   (`POST /servers/{id}/actions/rebuild`) and stays failed. This is lossy
   (minutes of downtime, anything written since the snapshot is lost) —
   accepted because a broken `harden.sh`/compose mutation has no clean undo.
6. **Always** prunes old CI snapshots, keeping the newest 5.

There is deliberately **no approval gate and no PR-time `tofu plan`**: a plan
against real state needs the same credentials as `apply`, which live in the
branch-restricted `production` Environment a PR can't reach. The
snapshot/restore is the safety net.

**If auto-restore didn't run** (e.g. job cancelled mid-apply): Hetzner console
→ the VPS → Snapshots → newest `ci-pre-apply` → Rebuild from Image, or the same
API call with `$HCLOUD_TOKEN` and an image id from
`curl https://api.hetzner.cloud/v1/images?type=snapshot&label_selector=purpose=ci-pre-apply`.

**One-time setup**, `production` Environment secrets, alongside the ones above:

```
HCLOUD_TOKEN                   (Hetzner Cloud API token, read+write)
INFRA_SERVER_ID                (same value as server_id in terraform.tfvars)
TF_STATE_R2_ACCESS_KEY_ID       (the scoped R2 token's Access Key ID)
TF_STATE_R2_SECRET_ACCESS_KEY   (the scoped R2 token's Secret Access Key)
TF_STATE_R2_ENDPOINT            (same value as in infra/backend.hcl)
```

`KAMAL_SERVER_IP` and `RESEND_API_KEY`/`EMAIL_FROM`/`NOTIFY_EMAIL_TO` (→
`TF_VAR_release_check_*`) are reused. Plus one repo-level **Variable**
(Settings → Secrets and variables → Actions → Variables; not sensitive):

```
INFRA_DEPLOY_SSH_PUBLIC_KEYS   (JSON array of literal key text, e.g.
                                '["ssh-ed25519 AAAA... me", "ssh-ed25519 AAAA... kamal-ci-deploy"]' —
                                same keys as deploy_ssh_public_keys, as text,
                                since CI can't read your ~/.ssh/*.pub files)
```

**Verify**: push a comment-only change to `infra/harden.sh`, confirm
`infra-apply` succeeds and a new `ci-pre-apply` snapshot appears in Hetzner.

## Cutover

Done. For reference, it was: point Cloudflare's apex A/AAAA records at the VPS
(leaving Resend's SPF/DKIM/DMARC untouched), and set `proxy.host`/`proxy.ssl`
plus `WEB_URL`/`API_URL` to `https://tools.xdoubleu.com`. kamal-proxy issues
the cert on its next boot; watch it with `ssh deploy@<ip> docker logs
kamal-proxy -f`. DigitalOcean App Platform is decommissioned.

## Migrate data from Supabase (one-time)

After `tofu apply` has stood up Postgres, stream a plain-SQL `pg_dump`
(including Supabase's `auth` schema) into `psql` on the VPS:

```bash
ssh deploy@<ip> docker ps   # confirm the postgres container's name

ssh deploy@<ip> "docker exec -i <container> psql --username=postgres --dbname=postgres" \
  <path-to-dump-file>
```

A PostgreSQL 17+ `pg_dump` may wrap the file in `\restrict`/`\unrestrict`
lines, which silently skip every other backslash command (including
`\if`/`\endif`). Strip them first:

```bash
sed -i '' '/^\\restrict /d; /^\\unrestrict /d' <path-to-dump-file>
```

The source database is untouched, so this is safe to re-run.

## Verify

```bash
ssh deploy@<ip>                    # should work, key auth only
ssh root@<ip>                      # should be rejected
sudo ufw status                    # on the box: only 22/80/443 open (deploy is sudo, not root)
sudo fail2ban-client status sshd   # jail active
systemctl is-active unattended-upgrades   # active
cat /etc/apt/apt.conf.d/20auto-upgrades   # both Periodic settings "1"
sudo unattended-upgrade --dry-run --debug # shows planned actions, no changes made

# Postgres: tunnel in (never exposed publicly)
ssh -L 5432:localhost:5432 deploy@<ip>
# in another shell, using the password from `tofu output -raw postgres_password`:
psql "postgres://postgres:<password>@localhost:5432/postgres" -c '\dt auth.*'
psql "postgres://postgres:<password>@localhost:5432/postgres" -c '\dn'
# auth_gotrue_legacy should not appear in \dn's output.

# Auth goes through the app itself: sign in with an existing account
curl -X POST https://tools.xdoubleu.com/api/auth.v1.AuthService/SignIn \
  -H 'Content-Type: application/json' \
  -d '{"email":"<existing-account-email>","password":"<...>"}'
```

## Destroy

```bash
tofu destroy <same -var flags as apply>
```

Only removes the firewall/attachment — the `null_resource`s have no
destroy-time provisioner, so containers, data volumes, and hardening stay on
the box. Delete the server itself in the console.
