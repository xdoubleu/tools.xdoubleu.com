# ADR-0020: A global email/Slack alert-delivery switch, configured in the UI

- Status: Superseded by #1530
- Issues: #1482
- Affects (historical): `api/internal/notifications/`, `api/internal/slack/`, `global.notification_channel_config`, `web/components/monitoring/NotificationChannelCard.tsx`

> **Superseded.** Alerting moved to Grafana (ADR-0022). With `ThresholdAlertJob`
> and `IssueNotifierJob` retired, the Slack client, the
> `global.notification_channel_config` table (dropped in `00049`), the
> `UpdateNotificationChannel` RPC, and the UI card are gone.
> `notifications.Service` now only has `EnqueueEmail` (digests) and `EnqueueTo`
> (family invites, feeds). Prod was already email-only.

## Context

#1482 asked for monitoring alerts (then email-only via Resend) to go to Slack
too.

## Decision (historical)

- One global `channel_mode ∈ {email, slack, both}` row, not per-source.
- The webhook URL was runtime config, sealed with `ENCRYPTION_KEY`, not a deploy
  secret; the RPC setting it was admin-gated because the URL is a credential.
- `notifications.Service.Enqueue` fanned out; `slack` mode fell back to email
  on failure so no alert was dropped. Weekly digests stayed email-only.

## Alternatives considered

- **Per-source channels** — config surface nobody asked for.
- **URL as deploy secret** — three-place secret list plus redeploy per rotation.
- **Slack bot token** — webhook suffices for one channel.

## Consequences

None remaining; kept for the reasoning. Grafana's contact points now own
delivery (ADR-0022).

## Revisit when

N/A — superseded.
