# ADR-0002: Split kobo-gateway's cache write into a `workflow_run` job, keep the artifact handoff

- Status: Accepted
- Issues: #1322, #1192, #1347
- Affects: `.github/workflows/build-kobo-gateway.yml`, `build-web.yml`, `save-kobo-gateway-cache.yml`, `main.yml`

## Context

kobo-gateway cannot be a Docker build stage at all: it needs cgo + the real
AppKit/Xcode SDK (see `kobo-gateway/cmd/kobo-gateway/menubar_darwin.go`'s
`//go:build darwin`), which no Linux container can produce. Its two output
files (the raw binary + `.dmg`) are never executed inside the deploy image
anyway — they are just served as static downloads. So it stays on the
`actions/cache`-restore-into-build-context mechanism while `api`/`web` moved to
BuildKit's own layer cache (see ADR-0003).

That left one job both writing a cache and running on untrusted input, which
CodeQL flagged as `actions/cache-poisoning/direct-cache` (#1322). An `if:`
gating the write by event/ref was tried first, and CodeQL kept flagging the
same rule against the same line across three different condition rewrites —
reading as this query not treating a runtime `if:` as a sanitizer at all.

## Decision

Caching this build is split across three workflows, **none of which both write
the cache and run on untrusted input**:

- **`build-kobo-gateway.yml`** (`workflow_dispatch`/`workflow_call`, so
  reachable from a PR — including a fork's) only ever **restores**
  `build-kobo-gateway-${{ hashFiles('kobo-gateway/**') }}`, building fresh on a
  miss, and uploads `kobo-gateway/dist/kobo-gateway` as the
  `kobo-gateway-dist` workflow-run artifact. It never writes the cache.
- **`build-web.yml`** downloads that same run's `kobo-gateway-dist` artifact
  instead of restoring the cache entry itself. This also removes a pre-existing
  race (#1192): the old cache-restore fired almost immediately while
  `build-kobo-gateway.yml`'s save only happened in its post-job step at the very
  end of its run. So `main.yml`'s `build-kobo-gateway`/`build-web` job gate is
  deliberately broadened to match `build-web`'s own trigger conditions (not just
  `kobo_gateway`/`workflows`), ensuring `build-kobo-gateway` always actually
  runs — never skips — whenever `build-web` needs its artifact. A
  source-unchanged run just cache-hits and skips the real compile, costing a
  macOS runner's boot time on a web-only change, not compile time.
- **`save-kobo-gateway-cache.yml`** is the only place that ever writes the
  cache. It triggers on `workflow_run` after a *completed* `Main Workflow` run
  (filtered in its own `if:` to `event == 'push'`) rather than being reachable
  directly from `pull_request`/`workflow_dispatch` — GitHub's documented pattern
  for deferring a privileged action past an untrusted pull request, since a
  `workflow_run` run always executes the workflow file version committed on the
  default branch, never a PR's. A cheap Linux `lookup-only` cache check runs
  first so the macOS runner underneath only spins up on an actual miss, keeping
  the cost profile the same as before this split for a push that doesn't touch
  `kobo-gateway/`.

## Alternatives considered

### Gating the cache write with an `if:` condition — rejected (#1322)

Tried first. CodeQL kept flagging the same rule against the same line across
three different condition rewrites, reading as this query not treating a runtime
`if:` as a sanitizer at all. The write moved out of every
`pull_request`/`workflow_dispatch`-reachable workflow instead.

### Publishing the build to a durable, addressable location — rejected (#1347)

A GitHub Release asset or a GHCR/GitHub Packages generic package with a stable
or SHA-versioned URL, so `build-web.yml` could `curl` it directly instead of
downloading `build-kobo-gateway.yml`'s workflow-run artifact, decoupling the two
jobs' scheduling. Rejected on inspection — it doesn't actually buy the
decoupling it promises, and reintroduces problems this pipeline already paid
down once:

- **It breaks PR-time integration validation, or reduces to the status quo.** A
  PR that changes `kobo-gateway/` needs `build-web.yml`'s Docker build to
  reflect *that PR's* kobo-gateway output, not whatever was last published from
  `main` — so the publish step could only safely run from a trusted,
  already-merged context (same as today's cache write), never from the PR's own
  run. That means a PR run still needs an in-run build of kobo-gateway for
  `build-web` to consume, which is exactly today's
  `upload-artifact`/`download-artifact` handoff — publishing a durable copy on
  top adds a second delivery mechanism without removing the first.
- **It buys no real latency win.** The "hard prerequisite" is already
  inexpensive on the common path: `build-kobo-gateway.yml`'s own cache
  (restored, not rebuilt, when `kobo-gateway/` is unchanged) makes a same-run
  `build-kobo-gateway` job that `build-web` waits on a ~20s no-op, not a real
  macOS compile — see this repo's own CI runs, where a cache-hit
  `build-kobo-gateway` job completes before `build-web` even starts pulling its
  artifact.
- **It re-opens the #1322 cache-poisoning class of problem in a new shape.** A
  publish step reachable from `pull_request`/`workflow_dispatch` that writes to
  any location a later, trusted run might read from is the same pattern CodeQL
  flagged for the `actions/cache` write — a Release asset or package tag would
  need the exact same "never write from an untrusted context, only from a
  `workflow_run`-after-push context" treatment `save-kobo-gateway-cache.yml`
  already implements for the cache, at which point it's the same trust boundary
  with more infrastructure (package/release tag naming and pruning,
  `packages: write` scope) layered on top.
- **It'd need to reinvent versioned lookup the cache/`RELEASE`-file combo
  already provides.** `build-kobo-gateway.yml`'s `dist/kobo-gateway/RELEASE`
  file plus its `hashFiles('kobo-gateway/**')` cache key already answer "what's
  the most recent actual build for this source tree" without any separate
  registry or "latest" pointer to keep consistent (a "latest" release/package
  tag has its own race: a push that touches only `kobo-gateway/` could be
  mid-publish when the next push's `build-web` run resolves "latest").

No workflow change was made for #1347; it is recorded here so it isn't
re-proposed and re-investigated from scratch.

## Consequences

- Never add a cache or artifact write to any workflow reachable from
  `pull_request` or `workflow_dispatch`. That is the invariant the whole split
  exists to hold.
- `build-web.yml` **cannot run standalone** via its own `workflow_dispatch` —
  see ADR-0003's consequences and `spec-ci-pipeline.md`.
- A web-only change still spins up a macOS runner (boot time, not compile time).
  That is the accepted price of guaranteeing the artifact exists.

## Revisit when

A real need for out-of-band kobo-gateway distribution shows up — e.g. serving
`.dmg` downloads from somewhere other than the `web` image. Revisit with that
concrete requirement driving the design, not build-dependency decoupling alone.
