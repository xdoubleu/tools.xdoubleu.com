# ADR-0004: `RELEASE` as a runtime container env for api/web; compile-time stamp only for kobo-gateway

- Status: Accepted
- Issues: #1038
- Affects: `api/internal/config`, `web/lib/env.ts`, `web/Dockerfile`, `.github/workflows/build-kobo-gateway.yml`, `build-web.yml`, `web/lib/books/gatewayClient.ts`, `web/components/Footer.tsx`

## Context

Every deployable needs to report which commit it came from, but the two kinds of
artifact here have different constraints. `api` and `web` run as containers we
control at deploy time. kobo-gateway is a `.dmg`/binary that **leaves the
container and runs on a user's own machine**, with no deploy-time environment to
read a version from.

## Decision

- **`api`/`web` do not bake the deploying commit's SHA into their compiled
  artifacts.** `RELEASE` is a plain container `ENV`, read at runtime
  (`api/internal/config`, `web/lib/env.ts`).
- **kobo-gateway's release stays compile-time-stamped**
  (`-X main.Release=...` in `build-kobo-gateway.yml`'s `make dist`).

Because kobo-gateway's build is cacheable (it skips `make dist` when
`kobo-gateway/` is unchanged), the release baked into the currently-bundled
artifact can legitimately be **older than the rest of the deploy**. So:

- `build-kobo-gateway.yml` records the SHA it actually used in a
  `dist/kobo-gateway/RELEASE` file (which survives the cache).
- `build-web.yml` reads that into a `KOBO_GATEWAY_RELEASE` build-arg, baked into
  `web/Dockerfile`'s image `ENV` directly.
- `gatewayNeedsUpdate` (`web/lib/books/gatewayClient.ts`) therefore compares a
  user's installed kobo-gateway against **what is actually bundled**, not this
  deploy's overall SHA. That comparison is what actually delivers *routine*
  releases (bug fixes that don't touch the protocol) to installed gateways.

## Alternatives considered

### One SHA for everything

Rejected: it would tell users their gateway is out of date whenever any part of
the repo deployed, and would claim a gateway is current when a cache-hit meant it
was never rebuilt. Both directions are wrong.

### Compile-time stamping for `api`/`web` too

Rejected: it forces an image rebuild for a value that changes on every deploy,
defeating layer caching for no benefit, since the container's environment is
fully under our control at run time.

## Consequences

- `web` and `api` can legitimately report **different** short release hashes.
  `web/components/Footer.tsx` surfaces both for exactly this reason — nothing
  forces them to match, and a mismatch is not a bug.
- `REQUIRED_GATEWAY_VERSION` in `gatewayClient.ts` remains a separate floor for
  genuine protocol breaks; bump it alongside `GatewayVersion` in the Go code
  only then. It is not a release-freshness mechanism.

## Revisit when

kobo-gateway gains a deploy-time environment of its own (e.g. it stops being a
user-installed binary), which would remove the reason for the split.
