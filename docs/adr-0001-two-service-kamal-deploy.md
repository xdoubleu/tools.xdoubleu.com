# ADR-0001: Deploy `api` and `web` as two independent Kamal services behind one proxy

- Status: Accepted
- Issues: #558, #904, #1038, #1029, #1034, #1113, #1132, #1106, #1111
- Affects: `config/deploy.api.yml`, `config/deploy.web.yml`, `.github/workflows/main.yml` (`deploy-kamal`), `api/cmd/api/kamal_proxy_shim.go`, `infra/`

## Context

The deploy topology went through three shapes:

1. **Split** originally.
2. **Merged into one container** (#558), because DigitalOcean App Platform
   billed per component. A hand-rolled `gateway/` Go module ran as PID 1 and
   provided the ingress split. Later split into 3 processes (#904).
3. **Split back into two independent services** (#1038), after #1029 migrated to
   a self-hosted Hetzner VPS and #1113 decommissioned DigitalOcean — removing the
   billing reason the merge existed at all, and with it the `gateway/` module.

## Decision

`api` and `web` each build their own Docker image (`api/Dockerfile`,
`web/Dockerfile`) and deploy as **two independent Kamal apps**
(`config/deploy.api.yml`, `config/deploy.web.yml`) to the Hetzner VPS, sharing
one kamal-proxy instance and one domain.

Routing is by kamal-proxy's own path-based routing:

- `api`'s config registers `proxy.path_prefix: "/api,/.well-known"`,
  **unstripped**. kamal-proxy can't strip only one of several prefixes on a
  single service, so `api/cmd/api/kamal_proxy_shim.go`'s `stripAPIPathPrefix`
  middleware does the `/api` stripping itself, leaving `/.well-known/*`
  (RFC 9728/8414 OAuth discovery) untouched.
- `web`'s config has no `path_prefix`, so it is the catch-all for everything
  else.

Since Cutover (#1034) this is *the* production deploy: `tools.xdoubleu.com`
resolves to the VPS, and both configs' `proxy.host`/`proxy.ssl` let kamal-proxy
terminate TLS with its own Let's Encrypt cert (shared for the one domain across
both services). `deploy-kamal` has no `continue-on-error` — a failure there fails
the workflow.

### Deploy order is required, not preferred

`deploy-kamal` runs two independent invocations, **web first, then api**:

```
bundle exec kamal deploy -c config/deploy.web.yml --skip-push --version=<sha>
bundle exec kamal deploy -c config/deploy.api.yml --skip-push --version=<sha>
```

kamal-proxy demands TLS be established by the root-path service (web, no
`path_prefix`) before a path-prefixed service (api) can register for the same
host. Reverse the order and the api deploy is rejected outright, or corrupts the
host's TLS state for web's later registration (#1132).

### Authentication and config

- SSH via an `ssh-agent` the job starts itself, loading a `KAMAL_SSH_KEY` repo
  secret through `printf '%s\n' | ssh-add -`. **#1106**: `webfactory/ssh-agent`
  fed the secret to `ssh-add -` verbatim, and a secret stored without its
  trailing newline made OpenSSH reject the PEM with `error in libcrypto`.
- `known_hosts` is seeded with a plain `ssh-keyscan` — **never `-H`**. **#1111**:
  SSHKit substitutes its own `known_hosts` reader for net-ssh's, and that one
  returns the keys of only the *first* matching hashed line; of keyscan's three
  lines only ssh-rsa counted as known, and the server's ed25519 key raised
  `Net::SSH::HostKeyMismatch`.
- **No config render step at all.** `config/deploy.api.yml`/`deploy.web.yml` are
  committed as-is, and Kamal evaluates each as ERB before parsing
  (`Kamal::Configuration.load_config_file`), so their only two non-committable
  values (`KAMAL_SERVER_IP`/`KAMAL_REGISTRY_USERNAME`) come straight from the
  job's env. #1113 removed the old `infra/templates/deploy.yml.tftpl` +
  `envsubst` indirection.
- Both are repo **Secrets**, not Variables: the repo is public, GitHub masks only
  Secrets, and `KAMAL_SERVER_IP` is echoed into an `ssh-keyscan` command.
- `deploy-kamal` runs under a GitHub `production` Environment
  (`environment: production`), branch-restricted to `main` with no required
  reviewers; its secrets live as environment secrets on `production` rather than
  repo-level. **The branch restriction is what actually matters** — only a `main`
  push can populate this job's `secrets.*` context; a PR run never can.

### What Tofu owns now

#1113 decommissioned DigitalOcean: the `deploy` job, `do-app.yaml`, and the
`DO_ACCESS_TOKEN`/`DO_APP_ID` secrets are gone, as is `infra/`'s duplicate local
deploy path (`null_resource.kamal_deploy`, `data.external.deployable_image`,
`deployable-image.sh`, and the 25 app-secret tfvars that fed it).

**Tofu now provisions the host only** — firewall, hardening, deploy keys,
Postgres — and never deploys the app. App secrets live in repo Secrets and
nowhere else. (GoTrue was part of Tofu's scope too, until #1039 replaced it with
first-party auth in `api` and removed the container entirely — see ADR-0005.)

## Alternatives considered

### Keeping the merged single-container deploy (#558)

Its only justification was DigitalOcean App Platform's per-component billing.
Once #1113 removed DigitalOcean, the merge cost a hand-rolled PID-1 ingress
process (`gateway/`) for no benefit, so both the merge and that module went.

### Stripping both path prefixes at the proxy

Not possible: kamal-proxy can't strip only one of several prefixes on a single
service, and `/.well-known/*` must reach the app unstripped for OAuth discovery
to work. Hence the in-app `stripAPIPathPrefix` middleware.

## Consequences

- Either service can be rolled back independently:
  `kamal rollback -c config/deploy.api.yml` / `-c config/deploy.web.yml`, without
  touching the other.
- The web-then-api ordering is load-bearing. Any change to `deploy-kamal` that
  parallelizes or reorders the two invocations will break TLS registration.
- `infra/README.md`'s "Automate Kamal deploys in CI" section is the single source
  of truth for the full secrets list.

## Revisit when

kamal-proxy gains per-prefix stripping (removes the shim), or the two services
need to scale/deploy on genuinely different cadences to different hosts.
