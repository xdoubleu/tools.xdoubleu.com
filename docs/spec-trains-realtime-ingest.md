# Spec: trains GTFS-Realtime ingest

- Source of truth: `api/apps/trains/` (`jobs.RealtimePollJob`,
  `services.RealtimeService`, `pkg/bmc.FetchRealtime`, `internal/models.Snapshot`)
- Issues: #1388, #1389, #1393

## Shape

`jobs.RealtimePollJob` runs every 30s (the BMC gateway's recommended cadence,
confirmed against real quotas in #1389) and delegates to
`services.RealtimeService.Poll`, which fetches `rt/trip-update` on every call
and `rt/alert` on a slower cadence (every 4th cycle, ~2 minutes — alerts
change far less often than delays) via `pkg/bmc.FetchRealtime`. Both feeds are
always requested as `?format=protobuf` and decoded with
`github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs`.

The decoded result replaces an in-memory `models.Snapshot` behind a mutex —
nothing here is persisted. A later slice of #1388 (6/7) reads
`RealtimeService.Snapshot()` to overlay delays onto the published timetable
and to resolve `Snapshot.Trips`' raw `trip_id` keys against the current
static import for display.

## Behavior

Each poll:

1. Fetch and decode `rt/trip-update` into `map[string]models.TripUpdate`,
   keyed by the feed's raw `trip_id`.
2. On roughly every 4th poll, also fetch and decode `rt/alert`; other polls
   keep the previous cycle's alerts.
3. Replace the snapshot wholesale with the new trips map, the current alerts,
   and the fetch timestamp.

A `bmc.RateLimitedError` (429) or `bmc.UpstreamError` with a 5xx status is
logged and the poll is skipped rather than failing the job — the next
scheduled run 30s later retries, which is the backoff #1393 asks for given a
cadence far below the gateway's quota (~5,760 of 12,000 daily requests at
30s/feed). Any other error — a decode failure, a 4xx, a non-protobuf
response — is returned so it is recorded in `global.job_runs` and reaches
Sentry via `observability.TrackedJob`, rather than failing silently.

### The trip-level / stop-level distinction

`schedule_relationship` means different things depending on where it
appears, and conflating them is the single most likely way to get this
wrong (an analysis that read stop-level `NO_DATA` as a cancellation once
reported 12,703 cancelled calls against a true figure of 48):

| Level | Value `1` | Value `2` | Value `3` |
|---|---|---|---|
| Trip (`TripDescriptor.ScheduleRelationship`) | `ADDED` | `UNSCHEDULED` | **`CANCELED`** |
| Stop time (`StopTimeUpdate.ScheduleRelationship`) | **`SKIPPED`** | **`NO_DATA`** | `UNSCHEDULED` |

`decodeTripUpdates`/`decodeStopCall` (`internal/services/realtime_decode.go`)
map these onto `models.DelayState`, a domain enum distinct from the raw GTFS-RT
values so no caller can collapse them by accident:

- Trip-level `CANCELED` → `models.DelayCancelled` on the `TripUpdate` itself.
- Stop-level `SKIPPED` → `models.DelaySkipped` on that one `StopCall` — the
  trip's own `State` stays whatever it already was (normally `DelayOnTime`).
- Stop-level `NO_DATA`, `UNSCHEDULED`, or a `SCHEDULED` call with neither an
  arrival nor a departure event → `models.DelayUnknown`.
- A `SCHEDULED` call with an arrival or departure delay of exactly `0` →
  `models.DelayOnTime`; a nonzero delay → `models.DelayDelayed`.

`models.DelayUnknown` is the default (`DelayState`'s zero value) precisely so
a caller that forgets to check state entirely fails toward "unknown", not
toward "on time" — in one observed sample 68% of calls carried `NO_DATA`,
making it the common case, not the edge case.

## Invariants

- **Never persist a `TripUpdate.TripID`** as a long-lived key — like the
  static feed's `trip_id` (`docs/spec-trains-gtfs-ingest.md`), it only makes
  sense for the lifetime of one `Snapshot`. A later slice must resolve it
  against the current static import before showing anything to a user.
- **A stop-level `NO_DATA`/`UNSCHEDULED` state must never be read as
  cancelled, skipped, or on time.** It is `models.DelayUnknown`, a fourth
  state distinct from all three.
- **`FetchRealtime` asserts the response `Content-Type` contains
  `protobuf`** and fails loudly otherwise, rather than parsing whatever came
  back — the gateway is documented (#1389) to serve JSON by default.
- A `bmc.RateLimitedError`/5xx `bmc.UpstreamError` must not fail the job; any
  other error must.

## Known gaps

- No persistence of any realtime state yet — `Snapshot` lives only in the
  process serving the poll job, so a restart or a second replica starts
  cold. Acceptable for now: the poll interval is 30s and the feed is wholly
  replaced every cycle regardless.
- Platform-change detection (a `stop_time_update.stop_id` moving between
  platform-level stops between polls) is not implemented — the data is
  present in the feed (#1389 confirmed the mechanism) but nothing here
  tracks it across polls yet; that belongs to whichever later slice of
  #1388 needs it.
- No overlay onto `SearchJourneys` yet — see
  `docs/spec-trains-journey-search.md` for the CSA planner this eventually
  attaches to.
