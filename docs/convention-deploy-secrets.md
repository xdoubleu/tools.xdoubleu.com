# Convention: a deploy secret is declared in three places that must agree

- Enforced by: `make lint/kamal-secrets` (`api/scripts/check_kamal_secrets.sh`), CI job `API Kamal Secrets Lint`
- Issues: #1390, #1404, #1405, #1468, #1504, #1507, #1509

## Rule

Every Kamal deploy secret must appear in **all three** of:

1. `config/deploy.api.yml` / `config/deploy.web.yml` / `config/deploy.grafana.yml`
   — the `env.secret:` list (three services since issue #1468 added Grafana)
2. `.kamal/secrets`
3. the matching `Deploy <svc> via Kamal` step's `env:` block in
   `.github/workflows/main.yml`

Adding a genuinely new secret also means creating the `production` Environment
secret — see `infra/README.md`, which is the single source of truth for the full
secrets list.

## Why

A name present in the first but missing from the others **only fails at
`kamal deploy` time on `main`** — post-merge, on an untested push, with the
deploy already underway:

```
Secret 'X' not found in .kamal/secrets
```

There is no earlier signal. The PR is green, the merge is clean, and the failure
lands in production deploy logs.

## Worked examples

`api/scripts/check_kamal_secrets.sh` cross-checks the three lists and fails the
PR when they disagree. `api-lint`'s gate in `main.yml` includes
`config_api`/`config_web` so a **config-only** PR actually runs it (#1405) —
without that, a PR touching only `config/deploy.*.yml` would skip the very check
that covers it.

## What violating it looked like

`BMC_PARTNER_KEY` shipped this way in #1390 and broke the `main` deploy; fixed in
#1404. #1405 then added the lint so it can't recur silently.

`config/deploy.grafana.yml` took three separate rounds to actually deploy,
none of them caught by any lint since CI has no way to run a real `kamal
deploy` or `kamal config` against the VPS:

1. #1468 shipped it with no `registry:` block at all, on the (wrong)
   assumption that Docker Hub needs no credentials for an anonymous
   public-image pull — Kamal's config schema requires
   `registry.username`/`password` unconditionally, the same requirement
   api/web's own configs already document for ghcr.io.
   `ConfigurationError: registry/username: is required`. #1504 "fixed" this
   with new `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secrets.
2. That surfaced a second bug: `image: grafana/grafana-oss:12.1.1` baked a
   tag in while the workflow also passed `--version` (the commit SHA) —
   Kamal concatenates `<image>:<version>` itself, producing the invalid
   `grafana-oss:12.1.1:<sha>`. `docker stderr: invalid reference format`.
   #1507 pinned `--version="12.1.1"` directly instead.
3. That surfaced a third bug: Kamal's `validate_image` step rejects any
   deployed image without a `service` docker label matching `service:` —
   the stock Docker Hub image obviously has none. `Image ... is missing the
   'service' label`. #1509 fixed this properly by giving Grafana its own
   built-and-pushed wrapper image (`infra/grafana.Dockerfile` +
   `build-grafana.yml`), exactly like api/web, pushed to this repo's own
   GHCR namespace — which also made the `DOCKERHUB_*` secrets from #1504
   unnecessary; they were reverted in the same change.

None of `registry:` credentials, an image/version mismatch, or a missing
`service` label are covered by `check_kamal_secrets.sh` — that check only
cross-references `env.secret:` entries. A future Kamal service deploying an
image this repo doesn't build itself needs all three verified by hand (or
`kamal config`, where available) since no lint catches any of them.
