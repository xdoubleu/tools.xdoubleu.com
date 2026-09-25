# ADR-0001: Deploy `api` and `web` as two independent Kamal services behind one proxy

- Status: Accepted
- Issues: #558, #904, #1038, #1029, #1034, #1113, #1132, #1106, #1111
- Affects: `config/deploy.api.yml`, `config/deploy.web.yml`, `.github/workflows/main.yml` (`deploy-kamal`), `api/cmd/api/kamal_proxy_shim.go`, `infra/`

## Context

`api` and `web` were once merged into one container (#558) behind a hand-rolled
`gateway/` PID-1 ingress, purely because DigitalOcean App Platform billed per
component. Moving to a self-hosted Hetzner VPS (#1029) and decommissioning DO
(#1113) removed that reason.

## Decision

`api` and `web` build their own images and deploy as **two independent Kamal
apps** sharing one kamal-proxy and domain, with kamal-proxy's Let's Encrypt
TLS.

- `api` registers `proxy.path_prefix: "/api,/.well-known"`, **unstripped**:
  kamal-proxy can't strip only one of several prefixes, so
  `kamal_proxy_shim.go`'s `stripAPIPathPrefix` strips `/api` in-process and
  leaves `/.well-known/*` (OAuth discovery) intact. `web` is the catch-all.
- **Deploy order is web, then api — required.** kamal-proxy needs the root-path
  service to establish TLS before a path-prefixed service can register for the
  same host; reversed, the api deploy is rejected or corrupts web's TLS state
  (#1132).
- CI SSH: `KAMAL_SSH_KEY` loaded via `printf '%s\n' | ssh-add -` (a secret
  missing its trailing newline fails with `error in libcrypto`, #1106).
  `known_hosts` is seeded with plain `ssh-keyscan`, **never `-H`** — SSHKit's
  reader only honors the first hashed line, so ed25519 raised
  `HostKeyMismatch` (#1111).
- No render step: configs are ERB, reading `KAMAL_SERVER_IP`/
  `KAMAL_REGISTRY_USERNAME` from env. Both are Secrets (the repo is public).
- Secrets live on the `production` Environment, branch-restricted to `main`, so
  a PR run can never read them.
- Tofu provisions the host only and never deploys; app secrets exist only as
  Environment secrets.

## Alternatives considered

- **Keep the merged container** — its only justification (per-component
  billing) is gone, and it cost a custom ingress process.
- **Strip both prefixes at the proxy** — kamal-proxy can't, and
  `/.well-known/*` must arrive unstripped.

## Consequences

- Each service rolls back independently (`kamal rollback -c config/deploy.<svc>.yml`).
- Parallelizing or reordering the two deploy invocations breaks TLS registration.
- `infra/README.md` ("Automate Kamal deploys in CI") holds the full secret list.

## Revisit when

kamal-proxy gains per-prefix stripping, or the services need different hosts or
cadences.
