# ADR-0010: Send the weekly digest as two emails, not one

- Status: Accepted
- Issues: #1014, #1253, #1355, #1214
- Affects: `api/internal/observability/jobs/` (`WeeklyDigestJob`), `api/cmd/api/main.go`

## Context

`jobs.WeeklyDigestJob` (#1014, `RunEvery = 7 days`, cross-app on `main.go`'s own
job queue) emails an admin a summary of everything still open. Unlike
`IssueNotifierJob` it has **no per-item dedup** — every run reports the current
state.

The content spans two unrelated domains: monitoring (unresolved Sentry issues,
failing `dependencies`-labeled PRs, open security alerts, slow transactions) and
feeds (feeds currently failing to poll, plus feeds with unread items).

## Decision

Send **two separate emails**, not one combined digest.

Monitoring and feeds are separate apps with separate audiences; folding both into
one mail left neither skimmable, filterable, nor mutable on its own.

- The **monitoring** email covers unresolved Sentry issues, failing
  `dependencies`-labeled PRs, open security alerts, and slow transactions.
- The **feeds** email covers feeds currently failing to poll (via
  `apps/feeds.Feeds.ListUnhealthy`, exposed the same way `BuildSummary` is) plus
  — #1355 — feeds with unread items (via `apps/feeds.Feeds.ListOpenItems`), gated
  by its own `NotificationSourceOpenFeedItems`/`open_feed_items` toggle so an
  admin who doesn't want the nudge can turn it off independently of the
  unhealthy-feeds source.

### All-clear and suppression rules

- Each email sends a **short all-clear** when a source feeding it is enabled but
  has nothing to report — so a *missing* weekly email is itself a sign the job
  stopped running.
- Each section is omitted when its source is disabled in
  `global.notification_settings` (#1214).
- An email is **suppressed entirely — not sent as an empty digest** — when every
  source feeding *that* email is disabled (#1253). The two emails decide this
  independently of each other.

## Alternatives considered

### One combined digest

The original shape. Rejected: with two audiences and two domains in one mail,
neither half could be skimmed, filtered by mail rules, or muted without muting
the other.

### Suppressing all-clears to reduce mail volume

Rejected: the all-clear is what makes the job's own liveness observable. Without
it, "no email" is ambiguous between "nothing wrong" and "the job died".

## Consequences

- Two emails per week per admin instead of one, by design.
- `main.go`'s `feedsHealthAdapter`/`feedsOpenItemsAdapter` bridge `*feeds.Feeds`
  into the job's own narrow `unhealthyFeedLister`/`openFeedItemsLister`
  interfaces, so this `internal/` package never imports an `apps/*` package.

## Revisit when

A third digest domain appears — at which point the per-email suppression logic
is worth generalizing rather than duplicating a third time.
