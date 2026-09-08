# ADR-0019: The trains router is an in-memory CSA index warmed off the request path, and the two GTFS feeds are correlated by (trip_short_name, service date)

- Status: Accepted
- Issues: #1388, #1390, #1391, #1393, #1394, #1484
- Affects: `api/apps/trains/`

## Context

The trains app ingests SNCB/NMBS data from two independent BMC endpoints:

- a **GTFS static** feed, imported daily and swapped into the `trains` schema
  atomically (`jobs.StaticImportJob`), and
- a **GTFS-Realtime** feed, polled every ~30s into an in-memory
  `models.Snapshot` (`jobs.RealtimePollJob` → `services.RealtimeService`).

Journey planning runs a Connection Scan Algorithm over an in-memory index
(`pkg/csa`) built from a rolling 14-day window of the static timetable — the
full feed is ~2.2M `stop_times` and only a window fits under the container's
300MiB `GOMEMLIMIT`. The realtime delay/cancellation/alert overlay is applied
on the live journey-detail page, on top of the planned legs, never inside
`SearchJourneys`.

Three properties of this shape are load-bearing and were each violated at least
once:

1. **`trip_id` is not a cross-feed key.** Both feeds carry a `trip_id`, but it
   is a daily-churning stopping-pattern variant assigned independently by each
   endpoint. The realtime overlay originally joined planned legs to live data
   by raw `trip_id` string equality (`snapshot.Trips[active.TripID]`), assuming
   the two feeds agreed. Nothing enforced that; when they diverge, every leg
   silently renders `DelayUnknown`, cancellations vanish, and there is no error
   or Sentry event (#1484).
2. **Building the index is not request-path work.** A cold `SearchJourneys`
   used to build the whole window index inline — a multi-query scan over the
   large `stop_times`/`calendar_dates` tables — with no deadline, exactly the
   silent-connection-reset failure mode of ADR-0017.
3. **`schedule_relationship` means different things at trip and stop level.**
   Conflating a stop-level `NO_DATA` with a trip-level `CANCELED` once produced
   12,703 false "cancelled" calls against a true figure of 48 (#1393); the
   `DelayState` enum keeps them distinct.

## Decision

**The CSA index lifecycle belongs entirely to background work.**
`services.JourneyService` builds the index in `Refresh`, guarded by a
`singleflight.Group` so the startup warm-up and the first scheduled
`RouterRefreshJob` tick share one build. `Trains.Start` kicks one warm-up in a
goroutine. `SearchJourneys` only *reads* `s.index`; while it is nil it returns
`services.ErrRouterWarmingUp`, which `connect_journeys.go` maps to
`CodeUnavailable`. It never builds the index itself.

**The two feeds are correlated by `(trip_short_name, service date)`, never by
`trip_id`.** `RealtimeService.Poll` resolves each decoded trip update's raw
`trip_id` against the current static import
(`FeedRepository.ShortNamesByTripIDs`) and keys `Snapshot.Trips` by
`models.TripKey{ShortName, Date}` — `Date` from the GTFS-RT `start_date`,
falling back to the poll-day Brussels date when the feed omits it. Updates
whose `trip_id` resolves to no static trip are dropped and counted in
`Snapshot.UnresolvedTripCount`, logged at WARN. `JourneyDetailService.buildLeg`
looks trains up with `snapshot.CallFor(tripShortName, serviceDate)` and never
handles a `trip_id`. `TripByShortNameOnDate` orders by `trip_id` so a short
name served by several pattern variants on a date resolves deterministically.

**`ImportParserVersion` gates re-import.** An import that starts writing a
column it previously did not must bump `services.ImportParserVersion` in the
same change, or the conditional GET keeps deployed rows pinned to the old
importer's output.

## Alternatives considered

- **Keep correlating by `trip_id`, add a consistency check.** Rejected: it
  still needs the same static-import lookup to know whether the ids agree, so
  it is strictly more work than keying by the resolved coordinates directly,
  and leaves the request path depending on an invariant no feed guarantees.
- **Correlate by `(route_id, start_date, stop pattern)`.** The GTFS-RT feed
  does carry `route_id` and `start_date`, but not `trip_short_name` — the field
  every user-facing surface already groups by. Matching on a stop pattern is
  fragile against the same daily churn. `trip_short_name` is the stable
  identifier the rest of the app is built on.
- **Lazily build the index in `SearchJourneys` but bound it with a context
  deadline.** Rejected: a deadline turns the unbounded build into a *reliably
  failing* request under load rather than a working one, and still spends the
  work N times for N concurrent cold callers. A background-owned index with a
  startup warm-up makes the cold window a few seconds, once.
- **Persist the realtime snapshot / a `trip_id` cross-walk table.** Rejected:
  the snapshot is wholly replaced every 30s and means nothing across a restart;
  a persisted cross-walk would need the same per-import rebuild and adds a
  schema for data with a 30-second lifetime.

## Consequences

- For a few seconds after a deploy (until the warm-up goroutine finishes)
  `SearchJourneys` returns `CodeUnavailable`; the web client must treat that as
  "retry shortly", not a hard error.
- `RealtimeService` now depends on the static import being present to
  correlate anything — on a fresh replica whose first static import has not run
  yet, every realtime trip update is "unresolved" and the overlay shows no live
  data. This is correct (there is no timetable to overlay onto) but means
  `UnresolvedTripCount` is briefly equal to the feed size at startup; only a
  *sustained* nonzero value indicates real feed drift.
- Alert-to-leg matching (`alertsForLeg`) still compares `alert.InformedTripIDs`
  against the static `trip_id`; alerts also match on route and stop, so the
  blast radius is smaller, but this is the same `trip_id` assumption and a
  candidate for the same treatment.
- Two RT trip updates resolving to the same `(trip_short_name, date)` — a feed
  anomaly — collapse to one entry (last wins).

## Revisit when

- BMC starts publishing `trip_short_name` (or a stable shared id) directly on
  the GTFS-RT `TripDescriptor` — then the static-import lookup in `Poll` can go
  away.
- The rolling-window index no longer fits in memory, or cold-start latency
  after a deploy becomes user-visible enough that a persisted/serialized index
  is worth the complexity.
