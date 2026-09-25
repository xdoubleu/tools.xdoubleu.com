# Convention: a deploy secret is declared in three places that must agree

- Enforced by: `make lint/kamal-secrets` (`api/scripts/check_kamal_secrets.sh`), CI job `API Kamal Secrets Lint`
- Issues: #1390, #1404, #1405, #1468, #1504, #1507, #1509, #1517, #1520

## Rule

Every Kamal deploy secret appears in **all three** of:

1. `config/deploy.{api,web,grafana}.yml` — `env.secret:`
2. `.kamal/secrets`
3. the matching `Deploy <svc> via Kamal` step's `env:` in `.github/workflows/main.yml`

A new secret also needs a `production` Environment secret; `infra/README.md`
holds the full list.

**Grafana names must be `GF_`-prefixed** (or ignored metadata: `RELEASE`,
`KAMAL_*`). Kamal injects names verbatim and Grafana reads only `GF_*`, so the
lint rejects anything else (#1520).

## Why

A mismatch fails only at `kamal deploy` on `main`, post-merge:
`Secret 'X' not found in .kamal/secrets`.

## Worked examples

`check_kamal_secrets.sh` cross-checks the lists. `api-lint`'s gate includes
`config_api`/`config_web` so a config-only PR still runs it.

## What violating it looked like

- `BMC_PARTNER_KEY` broke the `main` deploy (#1390, fixed #1404); the lint
  followed (#1405).
- `OAUTH_GRAFANA_CLIENT_SECRET` was consistent everywhere yet inert in Grafana
  until renamed `GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET` (#1517).
- Grafana's first deploy failed three times on things the lint doesn't cover: a
  missing `registry:` block (Kamal requires credentials even for public
  images), an image tag plus `--version` (Kamal appends `:<version>` itself),
  and no `service` label for `validate_image`. The fix was a repo-built wrapper
  image on GHCR (#1509). A future service deploying a third-party image needs
  all three checked by hand or with `kamal config`.
