# Convention: a deploy secret is declared in three places that must agree

- Enforced by: `make lint/kamal-secrets` (`api/scripts/check_kamal_secrets.sh`), CI job `API Kamal Secrets Lint`
- Issues: #1390, #1404, #1405

## Rule

Every Kamal deploy secret must appear in **all three** of:

1. `config/deploy.api.yml` / `config/deploy.web.yml` — the `env.secret:` list
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
