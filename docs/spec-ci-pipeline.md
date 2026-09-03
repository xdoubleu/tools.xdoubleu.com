# Spec: CI pipeline

- Source of truth: `.github/workflows/main.yml` and the reusable workflows it calls
- Issues: #863, #1405, #1036, #1038

## Shape

`.github/workflows/main.yml` orchestrates reusable workflows — `proto-check`,
`build-api`, `build-web`, `build-kobo-gateway`, `api-lint`, `web-lint`,
`kobo-gateway-lint`, `api-test`, `web-test`, `kobo-gateway-test` — gated by a
`changes` path filter.

`kobo-gateway-test`/`kobo-gateway-lint` run on a `macos-14` runner (they need to
compile cgo/AppKit).

`golangci-lint` runs across all three Go modules (`api`, `kobo-gateway`,
`sentrytools`) off one shared **root** `.golangci.yml` — golangci-lint's config
search walks up from the working directory, so the per-module
`working-directory` settings in `api-lint.yml`/`kobo-gateway-lint.yml`/
`sentrytools-lint.yml` all resolve to the same file.

## Behavior

### On PRs

Full suite runs; **`ci-pass` aggregates and is the required check**. It also
gates on Codecov's `codecov/project`/`codecov/patch` statuses — `codecov.yml`
sets `notify.manual_trigger: true` so Codecov posts nothing until `ci-pass` runs
`codecovcli send-notifications` after every test job uploads. (Upload count
varies by path filter, so a fixed `after_n_builds` doesn't work.)

### On push to `main`

`proto-check`/`api-lint`/`web-lint`/`kobo-gateway-lint` don't re-run — the PR's
green checks are trusted. `build-api`/`build-web`/`build-kobo-gateway` **do**
run: they are the sole producers of the deploy artifacts, not just PR-time
validation. Test jobs still run to refresh Codecov's baseline but never gate
deploy. `deploy-kamal` then deploys — see ADR-0001.

### Docs-only changes

The `api`/`web`/`kobo_gateway` filters exclude `**/*.md` (e.g. a `CLAUDE.md`-only
change doesn't set that filter's output), so a docs-only PR triggers none of the
build/lint/test jobs. `ci-pass` then has nothing to wait on and skips triggering
Codecov entirely — see its "Trigger and wait for Codecov to report" step, guarded
on at least one test job having actually run.

## Invariants

- **Never push directly to `main`** — `main` deploys without re-testing. Only
  merge PRs whose CI passed.
- When editing any `.github/workflows/*.yml`, its own `pull_request` trigger must
  include `.github/workflows/**` in its `paths` filter. Docker-build workflows
  are the deliberate exception — push-to-main only.
- The `kobo_gateway` path filter feeds `build-kobo-gateway` and `build-web`'s own
  gate — keep it in sync if `kobo-gateway/` moves.
- Never add a cache or artifact write to a `pull_request`/`workflow_dispatch`-
  reachable workflow (ADR-0002).

## Known gaps

### `build-web.yml` cannot run standalone (#1347)

Its "download `kobo-gateway-dist`" step only finds that artifact if a
`build-kobo-gateway` job already ran *in the same workflow run* — true when
`main.yml` calls it (it always runs `build-kobo-gateway` first via `needs:`),
never true for a bare manual dispatch, which has no other job to produce the
artifact and fails outright.

`build-web.yml` therefore no longer declares its own `workflow_dispatch` trigger
— `workflow_call` only, invoked exclusively through `main.yml`. `main.yml`'s own
top-level `workflow_dispatch` (which chains `build-kobo-gateway` before
`build-web` correctly) is the supported way to trigger a web build manually, at
the cost of also running every other job `main.yml` gates on `workflow_dispatch`.
There is no cheaper standalone entry point given the cross-job artifact
dependency. `build-api.yml` and `build-kobo-gateway.yml` have no such cross-job
input and remain safely dispatchable on their own.

### `ci-pass` fails with "Timed out waiting for Codecov to report" (#863)

Codecov's GitHub-app check-suite for the commit can get stuck permanently in
`queued` — confirm via:

```bash
gh api repos/<repo>/commits/<sha>/check-suites \
  --jq '.check_suites[] | select(.app.slug=="codecov")'
```

— even though Codecov's own backend finished processing the coverage report
(`https://api.codecov.io/api/v2/github/<owner>/repos/<repo>/commits/<sha>/`
shows `"state":"complete"`). No `codecov/patch`/`codecov/project` check-run is
ever created in that state, so `ci-pass` always times out (10 min) no matter how
many times the workflow job itself is rerun.

**There is no API to rerequest another app's check-suite with a PAT.** Push a new
commit (an empty one is fine) to get Codecov a fresh check-suite.
