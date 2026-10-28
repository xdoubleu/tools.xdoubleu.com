# ADR-0004: `RELEASE` as a runtime container env for api/web; compile-time stamp only for kobo-gateway

- Status: Accepted; since [ADR-0034](adr-0034-vite-spa-served-by-the-api.md) `web` ships inside the `api` image, so the SPA reads the api's runtime `RELEASE` through `/env.js`.
- Issues: #1038
- Affects: `api/internal/config`, `web/lib/env.ts`, `api/Dockerfile`, `.github/workflows/build-kobo-gateway.yml`, `build-api.yml`, `web/lib/books/gatewayClient.ts`, `web/components/Footer.tsx`

## Context

Every artifact reports its commit. `api`/`web` are containers with a deploy-time
environment; kobo-gateway runs on users' machines with none.

## Decision

- `api`/`web`: `RELEASE` is a runtime container `ENV`, not compiled in.
- kobo-gateway: compile-time `-X main.Release=...`. Its build is cached, so the
  bundled binary can be older than the deploy. `build-kobo-gateway.yml` writes
  the SHA it used to `dist/kobo-gateway/RELEASE`; `build-api.yml` passes it as
  the `KOBO_GATEWAY_RELEASE` build-arg; `gatewayNeedsUpdate` compares installed
  gateways against what's actually bundled.

## Alternatives considered

- **One SHA for everything** — would flag gateways stale on any deploy, and
  current when a cache hit meant no rebuild.
- **Compile-time stamp for api/web** — forces rebuilds and defeats layer
  caching for no benefit.

## Consequences

- `Footer.tsx` shows the web and api releases, which now come from one image;
  the bundled gateway's release can lag them, which isn't a bug.
- `REQUIRED_GATEWAY_VERSION` is a separate floor for protocol breaks only —
  bump it with `GatewayVersion` in Go.

## Revisit when

kobo-gateway gains a deploy-time environment.
