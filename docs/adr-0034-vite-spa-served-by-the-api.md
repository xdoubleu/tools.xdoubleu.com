# ADR-0034: Serve a Vite + React Router SPA from the api, with no Node service

- Status: Accepted
- Issues: #2555 (decided in #2553)
- Affects: `web/`, `api/internal/spa`, `api/cmd/api/spa.go`, `api/Dockerfile`, `config/deploy.api.yml`, `.github/workflows/main.yml`, `build-api.yml`, `infra/prometheus.yml`
- Supersedes: [ADR-0001](adr-0001-two-service-kamal-deploy.md)

## Context

The app was already client-rendered: data flows through SWR, IndexedDB, the
outbox and a service worker. Next.js added a second runtime (~170 MB steady),
the web-then-api deploy order and its skew workarounds, per-request rendering
for the CSP nonce, extra server renders for offline page saves, and a
`next build` of ~3 minutes in a ~5-minute PR run. Static export didn't fit: 23
pages sit under dynamic routes whose IDs are known only at runtime.

## Decision

- **`web/` is a Vite SPA** with React Router (data router, lazy page modules
  from `app/**/page.tsx`) and Vitest. Pages render their client components;
  nothing prefetches on a server.
- **The api serves the build** (`api/internal/spa`, enabled by `WEB_DIR`).
  It sits in front of the API handler and leaves `/api/*`, `/.well-known/*`,
  `/health`, `/metrics` and custom domains to it. Everything else is a static
  file or the HTML shell. It lives outside the api's rate limiter, so the
  precache of ~180 files can't trip it.
- **The api is the single root service** in kamal-proxy (`ssl`, no
  `path_prefix`); `kamal_proxy_shim.go` still strips `/api`. Grafana keeps
  `/grafana`. The web build is a stage of `api/Dockerfile`, with the
  kobo-gateway downloads in `public/downloads/`. There is no deploy order.
- **Server pieces moved to Go:** `/env.js` (replaces `window.__ENV__`
  injection), `/sw.js` with the `OFFLINE_DISABLED` kill switch, `/release`,
  `POST /logs` (log relay), `POST /web-vitals` (the `web_vitals_seconds`
  histogram, now on the api's `/metrics`), share-page `<title>`/noindex
  injection, and a static CSP (no nonce; PostHog's assets host is the only
  script host). The OAuth consent screen calls `/oauth2/authorize` with
  `Accept: application/json` and navigates to the returned `location`.
- **`/metrics` needs a bearer token** (`OBSERVABILITY_INGEST_SECRET`) because
  the api is now publicly reachable; Prometheus sends it. The `web` scrape job
  and `TargetMissing` for it are gone. `/api/observability/logs` is removed.
- **Offline:** the worker precaches the shell and every hashed file (the build
  lists them in `/sw.js`) and serves the shell for any navigation. Saved pages
  and the reader-chunk warmup are gone; data still comes from the persisted
  SWR cache and the outbox ([ADR-0027](adr-0027-offline-service-worker-and-persisted-swr.md)).
- **Sentry:** `@sentry/react` with the Vite plugin's source-map upload; the
  `/sentry-tunnel` route is dropped (`*.sentry.io` is in `connect-src`).

## Consequences

- A first visit on a new device shows loading placeholders until the API
  answers; repeat visits load from the offline cache.
- The api image carries the web build, so a web-only change rebuilds and
  redeploys the api, and build-kobo-gateway runs with every api build (cached).
- A one-time cutover is needed: kamal-proxy rejects a second root service, so
  `web` is removed from the VPS before the first deploy → `infra/README.md`.

## Alternatives considered

- **Next.js static export** — dynamic routes with runtime-only IDs.
- **React Router framework mode (`ssr: false`)** — a build toolchain for
  route conventions the glob already gives, plus a prerendered shell we don't
  need.
- **Keep Node for the server pieces** — the point was removing the runtime.
