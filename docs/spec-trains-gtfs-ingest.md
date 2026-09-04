# Spec: trains GTFS static ingest

- Source of truth: `api/apps/trains/` (`jobs.StaticImportJob`, `pkg/bmc`, `repositories.FeedRepository`)
- Issues: #1388, #1389, #1390

## Shape

`trains` is an SNCB/NMBS timetable app (schema `trains`, proto `trains.v1`).
#1390 added `jobs.StaticImportJob`; #1391 added `trains.v1.TrainService` and
its CSA journey planner (`docs/spec-trains-journey-search.md`) — still no
user-visible half of its own (that's #1388's slice 4). The realtime delay
overlay is a later slice of #1388.

The job is a daily download + validate + import of the Belgian Mobility Company
GTFS static feed via `pkg/bmc` (gateway host + `bmc-partner-key` from
`internal/config`).

## Behavior

The feed is swapped into the schema **in one transaction** —
`repositories.FeedRepository.ImportFeed` does `TRUNCATE` + `pgx.CopyFrom`, which
is atomic for readers under MVCC.

A conditional GET (`feed_info.etag`/`last_modified`) makes an unchanged daily
feed a no-op.

### Feed traps handled by the importer

Each of these came from the #1389 spike and each has a test:

- **`calendar.txt` weekday flags are an all-zero decoy** — service days resolve
  from `calendar_dates` alone.
- **`stop_times` values legitimately exceed `24:00:00`** — but a `36:00:00` bound
  rejects publisher-bug values (`87:39:00` observed).
- **CSV columns are alphabetically ordered** — parse by header name, never by
  position.
- **The download is verified a zip by magic bytes** (`PK\x03\x04`), not
  `Content-Type`.

## Invariants

- **`trip_id` is a daily-churning stopping-pattern variant.** Never persist it as
  a long-lived FK from user data; group user-facing output by
  `trips.trip_short_name`.
- `stop_times` (~0.8M rows) and `calendar_dates` (~1.07M rows) are large — see
  `convention-database-queries.md` before adding any list query over them.

## Known gaps

No realtime overlay yet — see `docs/spec-trains-journey-search.md` for the
journey planner added in #1391.
