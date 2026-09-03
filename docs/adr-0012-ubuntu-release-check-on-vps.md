# ADR-0012: Check for new Ubuntu releases with a local systemd timer, not an api job

- Status: Accepted (job removed)
- Issues: #1134
- Affects: `infra/` (systemd timer), formerly `api/internal/observability/jobs`

## Context

A prior `jobs.UbuntuReleaseJob` (#1134) polled Canonical's meta-release feed and
compared it against a **hardcoded baseline constant** that had to be bumped by
hand after every real `do-release-upgrade`.

Nobody did. So it fired a stale, wrong alert.

## Decision

The job was **removed** in favor of a systemd timer running locally on the VPS
that checks `do-release-upgrade -c` directly and emails via Resend.

The check therefore never depends on a hand-maintained constant, and no external
system needs to SSH into the box.

See `infra/README.md`'s "Getting notified of a new Ubuntu LTS release" section
for the actual unit.

## Alternatives considered

### Keeping the job and remembering to bump the constant

That was the status quo, and it failed in exactly the predictable way: the
constant is only wrong *after* a successful upgrade, which is the moment nobody
is thinking about the alerting code.

### Having the api SSH into the host to run the check

Rejected: it hands the application a shell on its own host for a cosmetic
notification. The timer runs where the answer already is.

## Consequences

- The check lives in infra, not application code, and is invisible to
  `get_job_stats` and the rest of the job observability surface.
- Losing the VPS's mail path loses the notification silently.

## Revisit when

Host-level checks multiply enough to justify a general host-agent, rather than one
timer per question.
