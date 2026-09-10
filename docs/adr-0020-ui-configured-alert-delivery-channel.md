# ADR-0020: A global email/Slack alert-delivery switch, configured in the UI

- Status: Superseded by #1530
- Issues: #1482
- Affects: `api/internal/notifications/`, `api/internal/slack/`,
  `api/internal/repositories/notification_channel_config.go`,
  `api/cmd/api/migrations/00042_notification_channel_config.sql`,
  `api/cmd/api/connect_observability_notifications.go`,
  `web/components/monitoring/NotificationChannelCard.tsx`

> **Superseded (issue #1530, Grafana Phase 4).** Alerting moved wholesale to
> Grafana — #1528 retired `ThresholdAlertJob` and #1541 retired
> `IssueNotifierJob`, which were the only producers that fanned out through
> `notifications.Service.Enqueue`. With no producer left, the whole delivery
> layer collapsed to email: the `slack` client, the
> `global.notification_channel_config` table (dropped in
> `00049_drop_notification_channel_config.sql`), the `UpdateNotificationChannel`
> RPC, and the `NotificationChannelCard` UI (already removed in #1547) are all
> gone. `notifications.Service` now only exposes `EnqueueEmail` (weekly digests)
> and `EnqueueTo` (family invites, feeds). Prod `channel_mode` was already
> `email`, so no live channel was dropped. The rest of this ADR is retained for
> historical context.

## Context

Every monitoring alert — `jobs.IssueNotifierJob`'s first-seen alerts and
`jobs.ThresholdAlertJob`'s breach/recovery transitions — went out over email
only, through `mailer` (Resend) to the one `NOTIFY_EMAIL_TO` address. #1482
asked for these to be able to land in a Slack channel instead of, or alongside,
email.

Two design axes had to be settled: *how much* configuration granularity (one
switch vs. per-source channel selection), and *where* the Slack webhook URL
lives (a deploy secret vs. runtime config).

## Decision

**One global switch.** `global.notification_channel_config` is a single row
with `channel_mode ∈ {email, slack, both}`, applied uniformly to every alert
producer. There is no per-source channel dimension on
`global.notification_settings` — that table stays a flat per-source on/off.

**Configured entirely through the monitoring settings UI.** The Slack Incoming
Webhook URL is stored in that same row (`slack_webhook_url BYTEA`), encrypted at
rest with the existing `crypto.Sealer` / `ENCRYPTION_KEY` — the same key that
protects OAuth tokens. It is **not** a deploy secret: nothing is added to
`config/deploy.api.yml`, `.kamal/secrets`, or `main.yml`. `GetNotificationSettings`
reports `channel_mode` and a `slack_webhook_configured` boolean but never the
URL itself; `UpdateNotificationChannel` is admin-gated (unlike the per-source
toggles, which are deliberately open — #1228) because the URL is a credential.

**Fan-out lives in `notifications.Service`.** `Enqueue` — the single chokepoint
every alert producer already calls — reads the switch and delivers to the
selected channel(s); no producer code changed. The per-channel outcomes are
folded back into the one error contract `OnResult` callbacks already expect
(`nil` if any channel delivered, `mailer.ErrNotConfigured` if every attempted
channel was merely unconfigured, the first real error otherwise), so the
existing dedup-key / alert-state persistence logic is untouched.

**Slack-only never silently drops an alert.** When `channel_mode == "slack"`
and Slack does not succeed — a send error *or* no webhook URL configured — the
alert also goes out over email as a fallback. `both` already sends email, so no
fallback applies there.

**Weekly digests stay email-only.** `WeeklyDigestJob` calls a separate
`notifications.Service.EnqueueEmail`, which bypasses the switch. The two digests
(→ [ADR-0010](adr-0010-two-weekly-digest-emails.md)) are long-form, skimmed at
leisure, and belong in an inbox, not a channel.

The Slack message is Block Kit (a header block + a section block), with the
section body truncated to Slack's 3000-character limit and a pointer to the
email/dashboard for the full text.

## Alternatives considered

**Per-source channel selection** (each alert source picks email/Slack/both).
Rejected: it multiplies the config surface (a `channel` dimension through the
table, the repo, the proto `NotificationSetting`, and the web toggle list) to
serve a need nobody expressed — the ask was "alerts in Slack", not "these
alerts here, those alerts there".

**Webhook URL as a deploy secret.** Rejected: it drags in the three-place Kamal
secret list and a redeploy for every rotation, to protect a value the app can
already encrypt at rest with a key it holds. Runtime config also lets a
non-deploying admin set it up.

**A Slack bot token (`chat.postMessage`).** Rejected: an Incoming Webhook is one
URL to one channel with no token lifecycle — exactly the shape of "send alerts
to our ops channel". Multiple channels or threading would justify a bot; that is
not on the table.

**Digests to Slack too.** Rejected for now: they are long, and a weekly wall of
text in a channel is noise where the same content in an inbox is a digest.

## Consequences

- `notifications.New` now takes a `slack.Client` and a channel-config getter.
  `notifications.NewEmailOnly` is the shim for tests and any future email-only
  caller; every existing test moved to it.
- `internal/notifications` now imports `internal/repositories` (for the config
  type and the `ChannelMode*` constants) and `internal/slack`. No cycle:
  `repositories` imports neither.
- The migration is `CREATE TABLE IF NOT EXISTS` + `INSERT … ON CONFLICT DO
  NOTHING` so the repository tests' mirrored-DDL `TestMain` and the real goose
  run can both land on the shared test database in either order.
- An admin who sets `channel_mode = "slack"` but never pastes a URL still gets
  every alert — over email, via the fallback. The fallback is invisible in the
  UI beyond the "no webhook URL configured yet" hint.
- Slack outages degrade to email only in `slack` mode; in `both` mode a Slack
  failure is swallowed as long as the email succeeded, matching how a Resend
  failure has always been handled.

## Revisit when

A second Slack destination is needed (per-team or per-severity routing), or the
digests genuinely need a channel presence — either would justify replacing the
webhook with a bot token and the single switch with real routing.
