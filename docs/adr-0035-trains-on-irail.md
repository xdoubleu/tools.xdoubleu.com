# ADR-0035: Back trains with the iRail API instead of our own GTFS pipeline

- Status: Accepted
- Issues: #2556 (decided in #2553)
- Affects: `api/apps/trains/`, `proto/trains/v1`, `web/components/trains`
- Supersedes: [ADR-0019](adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md)

## Context

For ~105 requests in 30 days, trains held 1.9 GB of Postgres (the largest
schema), polled the realtime feed every 30 s, and peaked the api heap near
1 GB during the static import. ADR-0019 also records repeated feed-correlation
bugs.

## Decision

`pkg/irail` replaces the GTFS import, realtime decoding and CSA router:

- `SearchStations`: iRail `stations` in nl/fr/en, cached in memory for a day.
- `SearchJourneys`: iRail `connections` (planned up to ~2 months ahead).
- `GetJourneyDetail`: iRail `vehicle` per leg. A cancelled stop on a running
  train is `skipped`; a train with every stop cancelled is `cancelled`.
- Journey break detection, re-planning, the websocket push and saved commutes
  stay. The push now polls only journeys that have not yet ended.
- Station ids are 9-digit UIC codes. Migration `00010` rewrites saved
  commutes from the `S`-prefixed GTFS ids and drops the GTFS tables.
- `journey_id` carries the iRail vehicle id, so old links stop resolving.
- The client sends a User-Agent naming the site, spaces requests under iRail's
  3 req/s limit, retries a 429 once, and caches responses for 20 s.
- `trains_get_irail_diagnostic` replaces the feed-info and realtime-trip
  tools: request/429/failure counts plus one train's raw response.

## Consequences

- No SLA: iRail is community-run. A failure surfaces as an error (429 as
  `unavailable`), never as invented data.
- Service alerts are not shown: iRail attaches them to connections, not to
  vehicles, so `LegDetail.alerts` is empty.
- No raw-feed debugging, and iRail sees every journey query.
- The `BMC_PARTNER_KEY` secret is gone.
- Attribution cites iRail instead of the NMBS-SNCB CC BY 4.0 feed notice.
