# Convention: fix the missing MCP tool before investigating the incident

- Enforced by: nothing but review
- Issues: #1027, #1195, #1214, #1357, #1374, #1377, #1424, #1453, #1459

## Rule

If a production issue has **no MCP tool that surfaces it**, or an existing tool
returns wrong/incomplete data, **fix that gap first** — add or correct the tool —
before investigating the issue itself. Then add a line to the log below.

## Why

Otherwise the same blind spot recurs. Every case below cost real investigation
time a working tool would have made unnecessary, and several were only
answerable with direct database access.

## Case log

- **#1027 — requests but not bytes.** Supabase restricted the project for
  blowing its egress quota and nothing could say which endpoint caused it.
  `global.usage_daily` gained a `bytes` column; `get_usage_stats` reports it.
  The database is billed per byte returned — see `convention-database-queries.md`.
- **#1195 — OAuth connection state was invisible.** A GitHub reconnect kept
  landing back on "Connect" and nothing reported why. `get_oauth_connections`
  now reports connected state plus requested, granted and required scopes — the
  three values that explain a not-connected verdict.
- **#1214 — no per-source notification toggle.** "Why didn't I get emailed" was
  unanswerable. `global.notification_settings` holds a per-source flag the jobs
  check before notifying, surfaced by `get_notification_settings`.
- **#1357 — project board columns were unreadable.** The GitHub MCP server's
  `list_issue_fields` only resolves custom fields on *organization* projects and
  this board is personal. `get_project_issues_by_status` queries GraphQL with
  the admin's own token instead. This added `read:project` to the GitHub OAuth
  scopes — **an admin who connected before that change must reconnect once.**
- **#1374 / #1377 — an unreconcilable games number.** The dashboard's completion
  rate disagreed with Steam's profile. Two blind spots: the
  `games_get_steam_distribution` `bucket` argument was documented as `0-9` while
  there are 11 buckets (fixed then), and nothing surfaced delisted games at all
  (not fixed then).
- **#1424 — the same delisted blind spot, three days later.** The rate
  disagreed again and the correct rule was nearly reverted for lack of evidence.
  `games_get_steam`'s `delisted` list now reports the excluded games, making the
  population behind a completion number checkable — see
  `adr-0018-completion-average-population.md`.
- **#1453 — the trains app had no tools at all.** Multilingual station names
  looked broken months after they landed; every layer of code was correct, so
  the question was what the database held, and `trains` had read RPCs but no
  `mcp.go`. Now `trains_search_stations`/`trains_get_feed_info`/
  `trains_search_journeys` exist, and `GetFeedInfo` carries `imported_at` —
  without it, "current" and "no import in weeks" look identical, since an
  unchanged feed keeps the same `feed_version`. The cause was a conditional GET
  making an importer change never re-import; `feed_info.parser_version` now
  forces a full re-import on a mismatch.
- **#1459 — an import that reported nothing about what it imported.** Same
  symptom a third time. A successful import applying zero translations and one
  of a monolingual feed emit identical logs. `feed_info` now stores
  `translation_rows`, `translation_rows_unmatched` and per-language
  `translated_stops_*`; `translated_stops_nl = 0` against a non-zero
  `translation_rows` names the failure directly. The bug was keying
  `translations.txt` rows by `record_id` alone when GTFS allows `record_id`
  **or** `field_value` — and the mock feed had been hand-written in the
  `record_id` shape, so **a fixture invented to match the code under test proves
  only that the code matches itself.**
