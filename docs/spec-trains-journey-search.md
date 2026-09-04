# Spec: trains journey search

- Source of truth: `api/apps/trains/pkg/csa` (the router), `api/apps/trains/internal/services/journey.go` (index lifecycle), `proto/trains/v1/trains.proto`
- Issues: #1388, #1390, #1391

## Shape

`trains.v1.TrainService.SearchJourneys` routes over the timetable
`jobs.StaticImportJob` (#1390) ingests, using a Connection Scan Algorithm
(CSA) over an in-memory index — not a DB query per search. Realtime delay
overlay and multimodal/walking legs are later slices of #1388; this slice is
scheduled-timetable-only.

## Rolling window, not the whole feed

The full feed is ~2.2M `stop_times` rows across a year. `JourneyService`
loads only a rolling window (`routerWindowDays` = 14 days) starting at
"today" in `Europe/Brussels`, rebuilt every `routerRefreshInterval` (6h) by
`jobs.RouterRefreshJob` so the window slides forward and picks up each
day's static import. This is what keeps the router inside the deployed
`api` container's `GOMEMLIMIT: 300MiB` (`config/deploy.api.yml`) — if a
future feed doesn't fit even a 14-day window, shrink the window further
before reaching for anything more complex (a precomputed transfer pattern,
a persistent index); 652 stops does not need either yet.

## Algorithm

`pkg/csa.Build` interns stops/trip-metadata to integer indices and produces
a `depTime`-sorted `[]connection` array — one elementary edge per
consecutive pair of `stop_times` rows on a (trip, service date) instance.
`SearchJourneys` runs a bounded number of full scans (`maxTransfersSearched`
+ 1 passes): pass *p* permits boarding one more new trip than pass *p-1*,
seeded from pass *p-1*'s arrival times, with same-station footpath
relaxation applied inline whenever a stop's arrival improves (so a platform
change is available to later connections in the same pass without costing
an extra pass). Each pass's best arrival at the destination becomes one
Pareto-frontier candidate; `paretoFilter` drops any candidate another
candidate dominates on both arrival time and transfer count, then the
result is capped at `maxJourneyResults` (5). A `SearchJourneys` call is
also how the frontend re-queries alternatives after a leg falls through:
pass the failed leg's stop as the new origin and its scheduled time as the
new `time`.

`arrive_by` is approximated, not run as a true reverse/profile search: it
scans forward from `when - arriveByLookback` (3h) and keeps only results
landing at or before `when`. `searchHorizon` (20h forward from the scan
start) bounds how far past the requested time a result can come from, so a
station pair with no near-term service returns empty instead of surfacing
an option days later.

## Correctness details

- **Transfers.** `transfers.txt` rows populate explicit footpaths
  (`transfer_type` 3 = not possible, excluded outright and never
  backfilled by the default; 1 = timed, 0 seconds; otherwise
  `min_transfer_time` or `defaultMinTransferSeconds` (180s)). Every other
  pair of distinct stops sharing a `parent_station` gets the default
  footpath unless `transfers.txt` already covers that exact pair — a
  same-station platform change is never free.
- **Non-boarding calls.** A `stop_times` row's `pickup_type`/`drop_off_type`
  gate boarding/alighting at that specific row, checked independently —
  the connection edge itself is still built through a non-boardable stop so
  a passenger already on the train keeps riding through it; the row is
  just never a leg boundary.
- **Service days** come from `calendar_dates` only, matching the importer
  (`docs/spec-trains-gtfs-ingest.md`) — `ActiveTripsInWindow` never touches
  `calendar.txt`.
- **After-midnight times.** `dayAbs` (a service date's abs-second offset)
  plus the stop_times seconds value (which may exceed 86400) is what makes
  a `28:00:00` departure land on the correct wall-clock day.
- **`trip_id`** is used only as an internal lookup key while building
  `connections` (`stopTimesByTripID`) — every `Leg`/`Journey` this package
  returns carries `trip_short_name`/`route_short_name` instead.

## Known gaps

No realtime overlay, no walking legs, no multimodal routing — all later
slices of #1388.
