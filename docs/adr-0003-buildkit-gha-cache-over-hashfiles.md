# ADR-0003: Use BuildKit's `type=gha` scoped cache, not hand-computed `hashFiles()` keys

- Status: Accepted
- Issues: #948, #900
- Affects: `.github/workflows/build-api.yml`, `build-web.yml`

## Context

`api` and `web` each build a multi-stage image with `docker buildx` and push to
their own GHCR repo. A hand-computed `hashFiles()` key shared across jobs
(#948) silently never hit: `build-web.yml` hashed `web/**` after `npm ci`,
folding in `node_modules`.

## Decision

- Cache with BuildKit's `type=gha,scope=<component>`; never reproduce a
  `hashFiles()` key across jobs.
- `build-api`/`build-web`/`build-kobo-gateway` are gated only on their own
  subtree (plus `proto`, `kobo_gateway` for web, and `.github/workflows/**`).
- Push `:latest` + `:<sha>` only on push to `main`; PRs build without pushing.

## Alternatives considered

- **Separate `docker-check` assembly job** (#900) — `build-push-action` already
  validates the build, and there's no merged image anymore (ADR-0001).
- **Shared `hashFiles()` keys** — fail silently, as above.

## Consequences

- `deploy-kamal` accepts a `skipped` build, but deploys a service only when its
  build produced a fresh `:<sha>`.
- `build-api.yml` uses `context: .` with `file: ./api/Dockerfile` for the
  `sentrytools` `replace` (ADR-0009).

## Revisit when

BuildKit's GHA backend is deprecated, or cache reuse becomes a measured
bottleneck.
