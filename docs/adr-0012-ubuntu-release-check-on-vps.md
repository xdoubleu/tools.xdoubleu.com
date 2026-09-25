# ADR-0012: Check for new Ubuntu releases with a local systemd timer, not an api job

- Status: Accepted (job removed)
- Issues: #1134, #1194
- Affects: `infra/` (systemd timer)

## Context

`jobs.UbuntuReleaseJob` compared Canonical's feed against a hardcoded baseline
that had to be bumped by hand after every upgrade. Nobody did, so it fired a
wrong alert.

## Decision

Removed in favor of a systemd timer on the VPS that runs `do-release-upgrade -c`
and emails via Resend — no constant to maintain, nothing SSHing in. See
`infra/README.md`'s "Getting notified of a new Ubuntu LTS release".

## Alternatives considered

- **Keep the job and bump the constant** — it goes stale exactly when nobody is
  looking.
- **api SSHes into the host** — hands the app a host shell for a notification.

## Consequences

- The check is invisible to `get_job_stats` and job observability.
- Losing the VPS's mail path loses the notification silently.

## Revisit when

Host-level checks multiply enough to justify a host agent.
