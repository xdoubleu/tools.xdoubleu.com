# Convention: fix the missing MCP tool before investigating the incident

- Enforced by: nothing but review
- Issues: #1027, #1195, #1214, #1357, #1374, #1377, #1397, #1424, #1453, #1459, #1554, #1564, #1616, #1818

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
- **#1453** — trains had no MCP tools. Added `trains_search_stations`,
  `trains_get_feed_info` (with `imported_at`), `trains_search_journeys`;
  `feed_info.parser_version` forces re-import on importer change.
- **#1459** — import reported nothing about translations. `feed_info` stores
  `translation_rows`, `translation_rows_unmatched`, per-language
  `translated_stops_*`. **A fixture written to match the code proves only that
  the code matches itself.**
- **#1397** — no live journey state. `trains_get_journey_detail` wraps
  `GetJourneyDetail`; current overlay only, no history.
- **#1554** — `prom_query` worked while `api`/`web` were never scraped.
  `TargetMissing` fires on `absent(up{job=...})`. **Answering the query you
  thought to ask is not coverage**; check that a new metric arrived.
- **#1564** — Grafana-managed alerts never appear in `ALERTS{}`.
  `get_grafana_alerts` reads the ruler API over the public URL.
- **#1616** — the lint checks secret names, not values; a malformed
  `OAUTH_OIDC_PRIVATE_KEY` crashed api at boot. Open.
- **#1818** — a slow job measured only end to end.
  `job_phase_duration_seconds` (`job_name`, `phase`) via
  `observability.ObserveJobPhase`, plus per-step Sentry spans. **Record phase
  splits when the steps exist, not when an incident forces it.**
