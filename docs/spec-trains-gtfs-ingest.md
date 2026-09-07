# Spec: trains GTFS static ingest

- Source of truth: `api/apps/trains/` (`jobs.StaticImportJob`, `pkg/bmc`, `repositories.FeedRepository`)
- Issues: #1388, #1389, #1390, #1450, #1453, #1459

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
feed a no-op — but only while `feed_info.parser_version` matches
`services.importParserVersion`. Those validators describe the **feed**, not
what the importer writes with it, so on a mismatch they are dropped and the
unchanged feed is fetched and imported in full (issue #1453).

**Bump `importParserVersion` in the same change whenever an import starts
writing something it previously did not** — a new column, a newly parsed file,
a changed derivation. Skipping the bump leaves the deployed rows pinned to
whatever the previous importer wrote for as long as SNCB publishes no new
feed, with no error anywhere: that is exactly how #1450's multilingual stop
names shipped as code and never reached the database. `feed_info.imported_at`,
surfaced by `GetFeedInfo` and `trains_get_feed_info`, is the signal that says
when an import last actually landed.

Each stop is stored under three names — `name_nl`/`name_fr`/`name_en`.
`stop_name` carries the feed's primary language (`feed_info.feed_lang`); the
optional `translations.txt` (`table_name=stops`, `field_name=stop_name`)
supplies the other two. A language with no translation entry falls back to the
primary `stop_name` (issue #1450).

**A `translations.txt` row is resolved to a stop three ways, in this order**
(issue #1459): by `record_id` matching the full `stop_id`; by `record_id`
matching the `stop_id` with the gateway's `gs:nmbssncb:` prefix stripped; and
by `field_value` matching the primary `stop_name`. GTFS makes `record_id` and
`field_value` mutually exclusive alternatives, so a parser handling only the
first silently translates nothing against a feed that uses the second — and
the bare-`record_id` case exists because BMC rewrites `stop_id` on the way out
while leaving `translations.txt` keyed by the unprefixed id. A `field_value`
match is on the *name*, so it reaches a station and all its platforms at once;
a `record_id` match reaches only the record it names.

`feed_info` stores what each import made of that file — `translation_rows`,
`translation_rows_unmatched`, and per-language `translated_stops_*` counts —
served by `GetFeedInfo`/`trains_get_feed_info`. **A feed with no
`translations.txt`, one whose rows match no stop, and a genuinely monolingual
one all render identically** (three copies of the same station name), so those
counts are the only thing that tells them apart: `translated_stops_nl = 0`
against a non-zero `translation_rows` is the signature of the #1459 failure.

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
- **`translations.txt` identifies rows by `record_id` *or* `field_value`** —
  see above; handling only one of the two fails silently, and the gateway's
  `stop_id` prefix rewrite means even `record_id` may not match verbatim.

## Invariants

- **`trip_id` is a daily-churning stopping-pattern variant.** Never persist it as
  a long-lived FK from user data; group user-facing output by
  `trips.trip_short_name`.
- `stop_times` (~0.8M rows) and `calendar_dates` (~1.07M rows) are large — see
  `convention-database-queries.md` before adding any list query over them.

## Known gaps

No realtime overlay onto the journey planner yet — see
`docs/spec-trains-journey-search.md` for the planner added in #1391 and
`docs/spec-trains-realtime-ingest.md` for the GTFS-Realtime poll job added in
#1393.
