# ADR-0044: Jobs queue on the server, the runner claims one at a time, silence means lost

- Status: Accepted
- Issues: #2706, #2701 (decisions 5, 7, 10, 15)
- Affects: `api/apps/factory/` (`internal/services/jobs.go`, `connect_jobs.go`,
  migration `00004`), `proto/factory/v1`, `api/cmd/api/routes.go` (`isLongRequest`)

## Context

Pressing Run on a task has to reach the owner's Mac without the Mac accepting any
inbound connection, survive the Mac being offline, and never let an agent approve
its own work ([ADR-0041](adr-0041-factory-runner-pairing.md)).

## Decision

- **A `jobs` table is the queue.** `RunTask` (a human admin in a browser session,
  as for Ready) adds a row and moves the task to In progress. The oldest queued
  row is claimed first, and a unique index allows one running job at a time. `StopJob` (same actor rule) cancels a queued or running job; the runner's next check-in finds it not running, which is how it learns to stop.
- **The runner pulls.** `ClaimJob` is held open for up to 20 seconds (a long
  route, [ADR-0017](adr-0017-long-request-handler-deadlines.md)) and polls the
  queue each second. `CheckIn` every 30 seconds keeps a job alive and tells the
  runner to stop when the job is no longer running. `CompleteJob` finishes it.
- **The prompt is frozen at Run.** It holds the repo's agent instructions, the
  task's title and body, and the Run text; the runner adds its own preamble and
  validates the repo and branch itself. Agent comments never enter it. If the
  task's body was last edited outside a browser session, Run is refused with a
  comment and the task goes to Needs you: an agent could have written text the
  owner never saw.
- **Five minutes of silence is lost.** A running job whose last check-in is that
  old becomes `lost` and its task moves to Needs you. No timer runs on the
  server; the sweep runs when a runner claims a job and when the board is read,
  so a dead runner is noticed the next time the owner looks.
- **Every finished job moves its task to Needs you** with the outcome as a
  comment, because without a PR (#2775) the branch stays in the Mac's clone.
- **The log is polled, not pushed.** The runner forwards the harness's stream-json
  lines every 2 seconds, secrets removed, to `AppendJobEvents`; `ListJobEvents`
  serves them to a human admin in a browser. A job keeps at most 5000 events, for
  30 days.
- The branch is `agent/<task-id>-<slug>`, chosen by the server and checked again
  by the runner.

- **Reply** queues a child job (`parent_job_id`) with the reply as its prompt, the
  parent's harness and branch. The runner receives the parent's session id as
  `resume_session_id`. It needs a finished newest job that left a session, and a
  task that isn't Done.

- **Slack** (`FACTORY_SLACK_WEBHOOK_URL`, https only; unset, nothing is posted) gets a
  message when a job opens a PR, fails, finishes without one, or is lost, and when a
  runner pairs. A message carries the task title as a link to `/factory?task=<id>`
  and the outcome's first line, escaped so a title can't mention the channel. It is
  sent in the background and never fails the work it reports.

## Consequences

- A lost job is only marked when something reads the board or a runner claims;
  a runner that never returns leaves the task In progress until then.
- A job's comment is written as `runner:<id>` or `system:factory`, so the board can
  tell it from a person's or an agent's.
- One job at a time means a long job blocks the queue; parallel jobs are out of
  scope.

## Revisit when

Jobs need cancelling (Stop, #2707), the queue needs more than FIFO, or a timer on
the server replaces the read-time sweep.
