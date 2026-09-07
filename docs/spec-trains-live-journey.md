# Spec: trains live journey view

- Source of truth: `api/apps/trains/` (`connect_journeys.go`'s
  `GetJourneyDetail`, `internal/services/journey_id.go`,
  `internal/services/journey_detail.go`, `internal/services/journey_ws.go`),
  `web/app/trains/[journeyId]`, `web/lib/trains/journeySocket.ts`
- Issues: #1388, #1394 (depends on #1392, #1393)

## Shape

A `SearchJourneys` result's `Journey.journey_id` is an opaque, self-describing
id — base64 of a JSON array of `{tripShortName, boardStopId, boardTime,
alightStopId, alightTime}` per leg (`services.EncodeJourneyID`/
`DecodeJourneyID`). Nothing is stored server-side: `GetJourneyDetail` decodes
the id back into those leg references, resolves each leg's `trip_short_name`
to that day's `trip_id` via `FeedRepository.TripByShortNameOnDate` (the same
`calendar_dates`-only resolution `ActiveTripsInWindow` uses), and rebuilds the
full stop-by-stop pattern between board and alight from `stop_times`,
overlaying `RealtimeService.Snapshot()`.

## Socket vs. polling decision

Reused `internal/communication/wstools` (the primitive `progressws` already
wraps for games/books job progress) rather than SWR polling, per the issue's
explicit steer — the repo built `wstools` for exactly this long-lived-push
shape, and a 30s realtime-poll cadence with potentially many concurrently
open journeys is inside what it already handles. `services.JourneyWSService`
registers one topic per journey id, created lazily by `GetJourneyDetail`
(`EnsureTopic`) rather than up front, since journey ids aren't known ahead of
time the way games/books' fixed job ids are. `RealtimeService.OnUpdate`
lets `JourneyWSService.PushAll` hook into every poll cycle and rebroadcast
fresh detail to every registered topic — a full snapshot each time, not a
diff, matching `progressws`' own state-broadcast shape. Topics are never
removed once created (a follow-up can add expiry once volume warrants it) —
an acceptable bound for a single long-lived process at this feature's scale.

The subscribe message and the pushed payload are the trains app's own JSON
DTOs (`models.JourneyDetail`'s `MarshalJSON`, mirroring the shape
`trains.v1.JourneyDetail`'s protobuf JSON encoding uses field-for-field) —
following `internal/progressws`' convention of a plain wire DTO rather than
protobuf-encoding a websocket payload.

## Reconnect after sleep

`web/lib/trains/journeySocket.ts`'s `useJourneyLive` is deliberately not a
naive auto-reconnect: on `document.visibilitychange` (to `visible`),
`window.pageshow`, or `window.online` it force-closes the current socket,
opens a fresh one, and calls the caller-supplied `refetchDetail` — not just
waiting for the next push. A phone locking its screen can suspend the socket
without firing a `close` event at all, so waiting on `onclose` alone can
leave a page silently stale indefinitely; the wake-signal listeners are the
actual guarantee here, `onclose`'s own reconnect timer is only the ordinary
drop/error path.

## Rendering states

`models.DelayState` (already the single source of truth for realtime states,
see `docs/spec-trains-realtime-ingest.md`) is what `StopDetail.State` reuses
directly — no second enum. `connect_journeys.go`'s `protoStopCall` renders it
via `DelayState.String()` into `trains.v1.StopCall.status`:
`"on_time"`/`"delayed"`/`"unknown"`/`"skipped"`/`"cancelled"`.
`web/components/trains/StopStatusBadge.tsx` maps each to a distinct label and
`Badge` variant — `"unknown"` must never render as `"on_time"`, and a
`delayed` reading under `onTimeThresholdSeconds` (60s) is folded into
`"on_time"` rather than shown as "delayed by 0". A whole-trip cancellation
(`LegDetail.Cancelled`) forces every one of that leg's stops to `"cancelled"`
regardless of any per-stop call.

## Invariants

- **`journey_id` never contains a raw `trip_id`** — only `trip_short_name`,
  which is resolved back to the day's `trip_id` on read, following the
  trip_id-churn invariant from #1390/#1393.
- **A stop with no realtime reading must render as `DelayUnknown`, never
  `DelayOnTime`.** `applyLiveState`'s zero-value `models.StopCall` already
  defaults to `DelayUnknown`, so "no call decoded for this stop_sequence" and
  "the call layer never ran" collapse to the same safe state by construction.
- **The websocket route is exempt from nothing** — per `api/CLAUDE.md`,
  `wstools.acceptWithHandshakeSpan` is the bounded signal; the connection's
  own long-lived transaction is expected to permanently breach
  `slow_transaction_http_high`, same as `progressws`.

## Known gaps

- Journey topics in `JourneyWSService` are never garbage-collected — a
  process that serves many distinct journeys over its lifetime grows the
  topic map unboundedly. Acceptable for now (issue #1394's scope); a
  follow-up can expire a topic once its journey's last leg's arrival time is
  comfortably in the past.
- No push notifications and no re-planning on a broken journey — both
  explicitly out of scope for this slice, see #1388's slice 7 for the latter.
