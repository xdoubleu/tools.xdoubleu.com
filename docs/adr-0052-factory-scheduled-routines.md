# ADR-0052: Routines run as scheduled factory jobs; enabling one is the approval

- Status: Accepted
- Issues: T-4, epic T-3
- Affects: `api/internal/platform/factory/`, `factory-runner/`, `api/internal/platform/identity/agentaccounts/`, `docs/convention-unattended-agent-trust.md`
- Changes: [adr-0025](adr-0025-machine-client-credentials-service-role.md) (routines stop using the `service` client), the Actions routine runner and its `automated_actions` ledger (retired)

## Context

The routines run on Actions with a GitHub App token in the agent's shell. The factory runs jobs only for a task you moved to Ready ([adr-0044](adr-0044-factory-jobs-queue.md)), in a VM that holds no GitHub credential ([adr-0043](adr-0043-factory-runner-sandbox.md), [adr-0045](adr-0045-factory-github-app.md)). A schedule has no Ready move, so something else must be the human approval.

## Decision

- **The server starts routines.** The existing `jobqueue` (durable through `job_runs`) enqueues a task-less routine job when one is due, unless that routine already has a job queued or running, so missed runs coalesce into one. The runner claims it like any job; Stop works.
- **The board holds what may run; `main` holds the prompt.** A routine's enabled flag, schedule, grants and harness are board settings that an admin or an agent account can change, the same actor rule as the Ready move (`CanManageRoutines`), through `factory_set_routine` and `factory_run_routine`. The change is announced with its author and stored in `updated_by`; a run's job carries its author in `created_by`. Resuming board-wide pickup needs an admin. Its prompt is its skill on `main`.
- **Enabling a routine is the approval.** Filing routines (nightly maintenance sweep, PostHog UX discovery) still write only to Backlog, so their output waits for your Ready move as before.
- **A routine job's own agent account** ([adr-0047](adr-0047-first-class-agent-users.md)), made at claim and revoked with the job like a task job's, with `task_id` the nil UUID. It holds `factory` (reads and comments board-wide and files tasks, but can't move or edit tasks; it lists, sets and runs routines, but not the one its own job runs for, and its Slack notices name the job and its task) plus an `observability` grant, which `AgentGrants` maps to the read tools and `resolve_sentry_issue`; `dismiss_security_alert` stays admin-only. A task job's account never holds the grant. The host mints a short-lived token per job; the VM reaches it only through the proxy's `/mcp` route.
- **No GitHub access in the VM.** GitHub reads go through the MCP tools. Third-party keys (PostHog) get a proxy route that swaps in the real key, like OpenRouter's.
- **Red-PR repair pushes through the host** (ADR-0045), narrowed: only to the head branch of an open, same-repo PR opened by Dependabot, Renovate, the owner or the factory App, never forced. Those PRs keep auto-merge. For a red `main` it opens a new PR **without** auto-merge and posts to Slack.
- **Red-PR repair runs on an event, not a schedule.** The factory App's `workflow_run` webhook queues it when a run fails on the default branch, on the repo's red-PR branch prefix, or for a bot. A queued or running job absorbs further failures.
- **The ready-issues executor retires.** Auto pickup already runs Ready tasks.
- **The Mac may sleep.** A due routine stays queued until a runner claims it; the existing "no runner for ten minutes" Slack notice covers it. A `pmset repeat wakeorpoweron` before the nightly run is optional.
- **No Actions fallback.** Every routine runs only as a factory routine: the Actions workflows, their `ROUTINE_<NAME>_ENABLED` variables and the `automated_actions` ledger with its `record_action`/`get_automated_actions` tools are gone, because factory jobs record their own runs. A red `main` is caught by the `red-pr-repair` routine's schedule, not by a CI-triggered run.
- **Harness:** the runner's own setting decides (epic T-269). On Claude Code, routines spend the Pro window, so schedule them outside working hours.

## Alternatives considered

- **Cron on the Mac:** the board couldn't see or stop it, and the schedule would live beside the hostile agent.
- **Routine definitions entirely in the repo:** an auto-merged PR could then enable a routine or widen its grants.
- **A second GitHub App for routines:** another key and installation for the same permissions; revoking routines alone isn't worth it.
- **Red-PR repair files a task instead of pushing:** a red dependency PR would wait for you, which the routine exists to avoid.

## Consequences

- The factory gains a task-less job type, and job accounts a routine scope beside `JobID`/`TaskID`.
- The watchdog, transcript upload and run ledger of the Actions runner are replaced by job check-ins, job events and Slack notices, not ported.

## Revisit when

Routines need to run while every Mac is off for long stretches, or a routine needs a write the host push can't express.
