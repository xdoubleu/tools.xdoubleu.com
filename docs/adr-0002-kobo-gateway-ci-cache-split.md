# ADR-0002: Split kobo-gateway's cache write into a `workflow_run` job, keep the artifact handoff

- Status: Accepted
- Issues: #1322, #1192, #1347
- Affects: `.github/workflows/build-kobo-gateway.yml`, `build-web.yml`, `save-kobo-gateway-cache.yml`, `main.yml`

## Context

kobo-gateway needs cgo + AppKit, so it can't be a Docker build stage; its
binary and `.dmg` are only served as static downloads from the `web` image. It
uses `actions/cache` rather than BuildKit's cache (ADR-0003). A job that both
wrote the cache and ran on untrusted input was flagged by CodeQL as
`actions/cache-poisoning/direct-cache` (#1322), and no `if:` gate satisfied it.

## Decision

No workflow both writes the cache and runs on untrusted input:

- **`build-kobo-gateway.yml`** (reachable from PRs) only **restores**
  `build-kobo-gateway-${{ hashFiles('kobo-gateway/**') }}`, builds on a miss,
  and uploads the `kobo-gateway-dist` artifact.
- **`build-web.yml`** downloads that run's artifact rather than the cache,
  fixing a restore-before-save race (#1192). `main.yml` gates
  `build-kobo-gateway` on `build-web`'s conditions so the artifact always
  exists.
- **`save-kobo-gateway-cache.yml`** is the only writer: `workflow_run` after a
  completed push-triggered `Main Workflow`, which always runs the default
  branch's workflow file. A Linux `lookup-only` check means the macOS runner
  starts only on a miss.

## Alternatives considered

- **`if:`-gated write** — CodeQL doesn't treat a runtime `if:` as a sanitizer.
- **Publish to a durable URL (Release/GHCR)** (#1347) — breaks PR-time
  validation of the PR's own build, gains no latency (a cache hit is ~20s), and
  reintroduces the same untrusted-write pattern.

## Consequences

- Never add a cache or artifact write to a workflow reachable from
  `pull_request` or `workflow_dispatch`.
- `build-web.yml` can't run standalone; dispatch `main.yml` instead.
- A web-only change still boots a macOS runner (boot time, not compile time).

## Revisit when

kobo-gateway needs distribution outside the `web` image.
