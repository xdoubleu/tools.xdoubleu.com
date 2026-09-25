# ADR-0010: Send the weekly digest as two emails, not one

- Status: Accepted; narrowed by #1597 (monitoring email removed)
- Issues: #1014, #1253, #1355, #1214, #1597
- Affects: `api/internal/observability/jobs/` (`WeeklyDigestJob`), `api/cmd/api/main.go`

## Context

`WeeklyDigestJob` (every 7 days, no per-item dedup) emails an admin everything
still open. Its content spanned two unrelated domains — monitoring and feeds —
which left one combined mail neither skimmable, filterable, nor mutable per
domain.

## Decision

- One email per domain. Only the **feeds** email remains: unhealthy feeds
  (`Feeds.ListUnhealthy`) and feeds with unread items (`Feeds.ListOpenItems`,
  own `open_feed_items` toggle, #1355).
- The **monitoring** email (Sentry, failing dependency PRs, security alerts,
  slow transactions) was removed in #1597 (migration `00051`): Grafana alerts
  on all four in real time (ADR-0022). Feeds has no Grafana equivalent.
- Each email sends a **short all-clear** when its enabled sources have nothing
  to report, so a missing email means the job stopped.
- A section is omitted when its source is disabled in
  `global.notification_settings` (#1214); the email is **suppressed entirely**
  when all its sources are disabled (#1253).

## Alternatives considered

- **One combined digest** — two audiences couldn't filter or mute separately.
- **No all-clears** — "no email" would be ambiguous with "job died".

## Consequences

- If a new unrelated digest domain appears, give it its own email rather than
  growing the feeds one.
- `main.go`'s `feedsHealthAdapter`/`feedsOpenItemsAdapter` bridge `*feeds.Feeds`
  to narrow interfaces so `internal/` never imports `apps/*`.

## Revisit when

A third digest domain appears — generalize the suppression logic then.
