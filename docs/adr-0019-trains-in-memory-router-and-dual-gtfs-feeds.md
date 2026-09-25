# ADR-0019: The trains router is an in-memory CSA index warmed off the request path, and the two GTFS feeds are correlated by (trip_short_name, service date)

- Status: Accepted
- Issues: #1388, #1390, #1391, #1393, #1394, #1484
- Affects: `api/apps/trains/`

## Context

Trains ingests two independent BMC feeds: **GTFS static**, imported daily and
swapped in atomically (`StaticImportJob`), and **GTFS-Realtime**, polled every
~30s into an in-memory `models.Snapshot` (`RealtimePollJob` →
`RealtimeService`). Journey search runs CSA (`pkg/csa`) over an in-memory index
of a rolling 14-day window (~2.2M `stop_times` won't fit under 300MiB
`GOMEMLIMIT` otherwise). The realtime overlay applies only on journey detail,
never in `SearchJourneys`.

Three properties were each violated once:

1. **`trip_id` is not a cross-feed key** — each endpoint assigns it
   independently and it churns daily. Joining on it silently rendered every leg
   `DelayUnknown` with no error (#1484).
2. **Building the index is not request-path work** — an inline cold build had
   no deadline (the ADR-0017 failure mode).
3. **Trip-level vs stop-level `schedule_relationship` differ** — conflating
   stop `NO_DATA` with trip `CANCELED` produced 12,703 false cancellations vs
   48 real (#1393). `DelayState` keeps them distinct.

## Decision

- **Background owns the index.** `JourneyService.Refresh` builds it under a
  `singleflight.Group`; `Trains.Start` warms it in a goroutine. `SearchJourneys`
  only reads it, returning `ErrRouterWarmingUp` (→ `CodeUnavailable`) while nil.
- **Correlate by `(trip_short_name, service date)`.** `RealtimeService.Poll`
  resolves each RT `trip_id` via `FeedRepository.ShortNamesByTripIDs` and keys
  `Snapshot.Trips` by `models.TripKey{ShortName, Date}` (`start_date`, else the
  Brussels poll date). Unresolved updates are dropped, counted in
  `UnresolvedTripCount`, and logged at WARN. `buildLeg` uses
  `snapshot.CallFor(...)`; `TripByShortNameOnDate` orders by `trip_id` for
  determinism.
- **Bump `services.ImportParserVersion`** whenever the importer starts writing a
  new column, or the conditional GET keeps old rows.

## Alternatives considered

- **`trip_id` plus a consistency check** — needs the same lookup, still relies
  on an unguaranteed invariant.
- **`(route_id, start_date, stop pattern)`** — patterns churn; `trip_short_name`
  is what the app groups by.
- **Lazy build with a deadline** — fails reliably under load and repeats work
  per cold caller.
- **Persist the snapshot or a cross-walk** — data with a 30s lifetime.

## Consequences

- Right after deploy `SearchJourneys` returns `CodeUnavailable` for seconds;
  clients retry.
- Before the first static import every RT update is unresolved; only a
  **sustained** nonzero `UnresolvedTripCount` means drift.
- `alertsForLeg` still matches `InformedTripIDs` on static `trip_id` (also on
  route/stop) — a candidate for the same fix.
- Two RT updates resolving to one key collapse (last wins).

## Revisit when

BMC publishes `trip_short_name` or a stable ID on the RT `TripDescriptor`, or the
window no longer fits in memory / cold start becomes user-visible.
