# Convention: fix the missing MCP tool before investigating the incident

- Enforced by: nothing but review
- Issues: #1027, #1195, #1214, #1357, #1374, #1377, #1397, #1424, #1453, #1459, #1554, #1564, #1616, #1818, #2034, #2226, #2365

## Rule

If a production issue has **no MCP tool that surfaces it**, or a tool returns
wrong or incomplete data, **fix the tool first**, then investigate. Add a line
to the log below.

## Why

Otherwise the blind spot recurs. Each case below cost investigation time a
working tool would have saved, often requiring direct database access.

## Case log

- **#1027** — egress quota blown, no per-endpoint bytes. `usage_daily.bytes`;
  `get_usage_stats` reports it.
- **#1195** — OAuth reconnect loop unexplained. `get_oauth_connections` reports
  requested, granted, and required scopes.
- **#1214** — "why no email?" unanswerable. Per-source flags in
  `global.notification_settings`, via `get_notification_settings`.
- **#1357** — personal project board unreadable by GitHub MCP.
  `get_project_issues_by_status` uses GraphQL with the admin token; added
  `read:project` (**admins connected earlier must reconnect once**).
- **#1374/#1377/#1424** — completion rate vs Steam unreconcilable.
  `games_get_steam`'s `delisted` list exposes the excluded population
  (ADR-0018).
- **#1453** — trains had no MCP tools. Added `trains_search_stations` and
  `trains_search_journeys`.
- **#1459** — the GTFS import reported nothing about translations.
  **A fixture written to match the code proves only that the code matches
  itself.**
- **#1397** — no live journey state. `trains_get_journey_detail` wraps
  `GetJourneyDetail`; current overlay only, no history.
- **#1554** — `prom_query` worked while `api`/`web` were never scraped.
  `TargetMissing` fires on `absent(up{job=...})`. **Answering the query you
  thought to ask is not coverage**; check that a new metric arrived.
- **#1564** — Grafana-managed alerts never appear in `ALERTS{}`.
  `get_grafana_alerts` reads the ruler API over the public URL.
- **#1616** — the lint checks secret names, not values; a malformed
  `OAUTH_OIDC_PRIVATE_KEY` crashed api at boot. `deploy-kamal` now runs
  `openssl rsa -check` on it before deploying (#2412).
- **#1818** — a slow job measured only end to end.
  `job_phase_duration_seconds` (`job_name`, `phase`) via
  `observability.ObserveJobPhase`, plus per-step Sentry spans. **Record phase
  splits when the steps exist, not when an incident forces it.**
- **#2034** — `get_failing_pull_requests` returned bare `{"configured":true}`:
  protojson drops empty lists, `claude/` PRs were filtered out, and pages were
  capped at 30. It now emits defaults, matches red-pr-repair's scope, pages,
  and errors on upstream failure. **Empty must look different from broken.**
- **#2226** — running trains showed "No live data" with no way to tell a
  missing realtime trip from a failed stop match. After the move to iRail
  (ADR-0035), `trains_get_irail_diagnostic` returns request, 429 and failure
  counts plus one train's raw iRail response.
- **#2365** — Kobo store-book progress never reached the library, with no
  error: the mirror dropped unmatched or 0% states silently.
  `books_list_kobo_store_books` lists each store book's library match and the
  last mirrored percent with its outcome (`mirrored`, `no_library_match`,
  `no_progress`, `not_newer`).
- **#2412** — every api boot rejected `OAUTH_OIDC_PRIVATE_KEY`, but
  `get_logs` showed `{"error": {}}`: the log tee marshalled `error`,
  `LogValuer` and group attrs as empty objects. They now keep their text. The
  cause was Kamal's env file escaping the PEM's newlines.
