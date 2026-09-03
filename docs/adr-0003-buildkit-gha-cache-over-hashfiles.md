# ADR-0003: Use BuildKit's `type=gha` scoped cache, not hand-computed `hashFiles()` keys

- Status: Accepted
- Issues: #948, #900
- Affects: `.github/workflows/build-api.yml`, `build-web.yml`

## Context

`api` and `web` each build their own image (`api/Dockerfile`, `web/Dockerfile` —
real multi-stage builds ending in a runnable final stage, deployed directly
rather than assembled into a merged image) and push to their own public GHCR
repo (`ghcr.io/xdoubleu/tools.xdoubleu.com/{api,web}`), built via
`docker buildx build`.

An earlier attempt (#948) reproduced a hand-computed `hashFiles()` cache key
across jobs. It broke: `build-web.yml` computed `hashFiles('web/**')` *after*
`npm ci`, folding the gitignored `node_modules` tree into the key, which only
ever self-matched within that one job.

## Decision

Cache via BuildKit's own `type=gha,scope=<component>` layer cache. **No
hand-computed `hashFiles()` cache key is reproduced across jobs anywhere in this
path.**

Job gating: `main.yml`'s `build-api`/`build-web`/`build-kobo-gateway` jobs are
each gated only on their **own** subtree changing (`api`/`proto` for
`build-api`; `web`/`proto`/`kobo_gateway` for `build-web`; `kobo_gateway` for
`build-kobo-gateway`; any of them also run on a `.github/workflows/**` change) —
not on any image-affecting change generally.

Each pushes `:latest` + `:<sha>` (full commit SHA) only on push to `main`. PR
runs build (validating the image actually compiles) but never push.

## Alternatives considered

### A separate merged-image `docker-check` assembly job — no longer applicable (#900)

`docker/build-push-action@v7`'s own step already provides the PR-time build
validation that a separate assembly job used to. #900's original motivation no
longer applies once there is no merged image (see ADR-0001).

### Hand-computed `hashFiles()` keys shared across jobs — rejected (#948)

See Context. Ordering relative to dependency installation makes the key
unreliable in a way that fails silently: it still "works", it just never hits.

## Consequences

- `deploy-kamal` accepts `skipped` as well as `success` for
  `needs.build-api.result`/`needs.build-web.result`, since a build job may
  legitimately not run — but only runs a given service's `kamal deploy` step
  when that service's own build job produced a fresh `:<sha>` tag, since a
  skipped build means nothing changed for that service and there is no new tag
  to deploy.
- `build-api.yml` builds with `context: .` (repo root) plus an explicit
  `file: ./api/Dockerfile`, because `api`'s local `replace` of `sentrytools`
  falls outside an `./api` context — see ADR-0009.

## Revisit when

BuildKit's GHA cache backend is deprecated, or cross-job cache reuse becomes a
measured bottleneck rather than an assumed one.
