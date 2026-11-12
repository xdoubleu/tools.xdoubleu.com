# ADR-0052: Routines run as scheduled factory jobs; enabling one is the approval

- Status: Accepted
- Issues: T-4, epic T-3
- Affects: `api/apps/factory/`, `factory-runner/`, `api/internal/agentaccounts/`, `.github/workflows/routine-*.yml`, `docs/convention-unattended-agent-trust.md`
- Changes: [adr-0025](adr-0025-machine-client-credentials-service-role.md) (routines stop using the `service` client), [spec-agent-routines-on-actions](spec-agent-routines-on-actions.md) (retired per routine at cut-over)

## Context

The routines run on Actions with a GitHub App token in the agent's shell. The factory runs jobs only for a task you moved to Ready ([adr-0044](adr-0044-factory-jobs-queue.md)), in a VM that holds no GitHub credential ([adr-0043](adr-0043-factory-runner-sandbox.md), [adr-0045](adr-0045-factory-github-app.md)). A schedule has no Ready move, so something else must be the human approval.

## Decision

- **The server starts routines.** The existing `jobqueue` (durable through `job_runs`) enqueues a task-less routine job when one is due, unless that routine already has a job queued or running, so missed runs coalesce into one. The runner claims it like any job; Stop works.
- **The board holds what may run; `main` holds the prompt.** A routine's enabled flag, schedule, grants and harness are board settings that only a human admin in a browser can change, the same actor rule as Ready and Run. Its prompt is its skill on `main`.
- **Enabling a routine is the approval.** Filing routines (nightly maintenance sweep, PostHog UX discovery) still write only to Backlog, so their output waits for your Ready move as before.
- **A standing agent account per routine** ([adr-0047](adr-0047-first-class-agent-users.md)) with fixed grants: `factory` create and comment plus read-only observability tools, and for red-PR repair `get_failing_pull_requests` and `get_workflow_runs`. The host mints a short-lived token per job; the VM reaches it only through the proxy's `/mcp` route.
- **No GitHub access in the VM.** GitHub reads go through the MCP tools. Third-party keys (PostHog) get a proxy route that swaps in the real key, like OpenRouter's.
- **Red-PR repair pushes through the host** (ADR-0045), narrowed: only to the head branch of an open, same-repo PR opened by Dependabot, Renovate, the owner or the factory App, never forced. Those PRs keep auto-merge. For a red `main` it opens a new PR **without** auto-merge and posts to Slack.
- **The ready-issues executor retires.** Auto pickup already runs Ready tasks.
- **The Mac may sleep.** A due routine stays queued until a runner claims it; the existing "no runner for ten minutes" Slack notice covers it. A `pmset repeat wakeorpoweron` before the nightly run is optional. `AutomatedRoutineMissed` reads each routine's last job, alerting after 48 hours.
- **Cut-over one routine at a time.** Each routine is enabled on exactly one side: the `ROUTINE_<NAME>_ENABLED` variable on Actions or the board toggle, never both. Flipping back to Actions is the fallback while the Mac is away; the Actions routines are deleted after 30 days without a fallback.
- **Harness:** the runner's own setting decides (epic T-269). On Claude Code, routines spend the Pro window, so schedule them outside working hours.

## Alternatives considered

- **Cron on the Mac:** the board couldn't see or stop it, and the schedule would live beside the hostile agent.
- **Routine definitions entirely in the repo:** an auto-merged PR could then enable a routine or widen its grants.
- **A second GitHub App for routines:** another key and installation for the same permissions; revoking routines alone isn't worth it.
- **Red-PR repair files a task instead of pushing:** a red dependency PR would wait for you, which the routine exists to avoid.

## Consequences

- The factory gains a task-less job type, and job accounts a routine scope beside `JobID`/`TaskID`.
- The watchdog, transcript upload and `record_action` ledger of `agent-routine.yml` are replaced by job check-ins, job events and Slack notices, not ported.
- A skill change no longer ships on green CI alone.

## Revisit when

Routines need to run while every Mac is off for long stretches, or a routine needs a write the host push can't express.
