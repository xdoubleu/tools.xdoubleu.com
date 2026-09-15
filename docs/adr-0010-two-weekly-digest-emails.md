# ADR-0010: Send the weekly digest as two emails, not one

- Status: Accepted; narrowed by #1597 (the monitoring email was removed — see
  "Narrowed" below)
- Issues: #1014, #1253, #1355, #1214, #1597
- Affects: `api/internal/observability/jobs/` (`WeeklyDigestJob`), `api/cmd/api/main.go`

## Context

`jobs.WeeklyDigestJob` (#1014, `RunEvery = 7 days`, cross-app on `main.go`'s own
job queue) emails an admin a summary of everything still open. It has **no
per-item dedup** — every run reports the current state. (The realtime
first-seen notifier that did dedup, `IssueNotifierJob`, was retired in #1541;
Grafana's `service-health` alert group covers first-seen alerting now →
[`adr-0022`](adr-0022-prometheus-grafana-metrics.md).)

The content originally spanned two unrelated domains: monitoring (unresolved
Sentry issues, failing `dependencies`-labeled PRs, open security alerts, slow
transactions) and feeds (feeds currently failing to poll, plus feeds with
unread items).

## Decision

Send **two separate emails**, not one combined digest.

Monitoring and feeds are separate apps with separate audiences; folding both into
one mail left neither skimmable, filterable, nor mutable on its own.

- The **monitoring** email covered unresolved Sentry issues, failing
  `dependencies`-labeled PRs, open security alerts, and slow transactions.
  Removed in #1597 — see "Narrowed" below.
- The **feeds** email covers feeds currently failing to poll (via
  `apps/feeds.Feeds.ListUnhealthy`, exposed the same way `BuildSummary` is) plus
  — #1355 — feeds with unread items (via `apps/feeds.Feeds.ListOpenItems`), gated
  by its own `NotificationSourceOpenFeedItems`/`open_feed_items` toggle so an
  admin who doesn't want the nudge can turn it off independently of the
  unhealthy-feeds source.

### All-clear and suppression rules

- The remaining email sends a **short all-clear** when a source feeding it is
  enabled but has nothing to report — so a *missing* weekly email is itself a
  sign the job stopped running.
- Each section is omitted when its source is disabled in
  `global.notification_settings` (#1214).
- The email is **suppressed entirely — not sent as an empty digest** — when
  every source feeding it is disabled (#1253).

## Narrowed (#1597)

The monitoring email's four sections (Sentry, failing dependency PRs, security
alerts, slow transactions) were dropped from `WeeklyDigestJob` and their
`global.notification_settings` rows deleted (migration
`00051_drop_redundant_digest_sources.sql`). By the time #1597 shipped, adr-0022
had already moved all four to real-time Grafana alerting delivered to Slack
(Phase 2/3/9), making the weekly restatement purely redundant — unlike the
feeds sections, which have no Grafana equivalent since Grafana only watches
infra metrics, not per-user reading/feed-backlog state.

`WeeklyDigestJob.Run` now builds and sends a single email (feeds: unhealthy
feeds + open items). The "two emails, not one" title and the
independent-per-email suppression/all-clear rules above are kept as this ADR's
record of *why* the two domains were ever split — should a second, unrelated
digest domain appear again, split it back out along the same reasoning rather
than growing the surviving feeds email into a second monitoring-shaped thing.

## Alternatives considered

### One combined digest

The original shape. Rejected: with two audiences and two domains in one mail,
neither half could be skimmed, filtered by mail rules, or muted without muting
the other.

### Suppressing all-clears to reduce mail volume

Rejected: the all-clear is what makes the job's own liveness observable. Without
it, "no email" is ambiguous between "nothing wrong" and "the job died".

## Consequences

- One email per week per admin now, not two — the monitoring email's audience
  (Sentry/CI/security/perf issues) gets that same information from Grafana's
  real-time Slack alerts instead.
- `main.go`'s `feedsHealthAdapter`/`feedsOpenItemsAdapter` bridge `*feeds.Feeds`
  into the job's own narrow `unhealthyFeedLister`/`openFeedItemsLister`
  interfaces, so this `internal/` package never imports an `apps/*` package.

## Revisit when

A third digest domain appears — at which point the per-email suppression logic
recorded above is worth generalizing rather than duplicating a third time.
