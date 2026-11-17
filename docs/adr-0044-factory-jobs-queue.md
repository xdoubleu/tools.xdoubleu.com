# ADR-0044: Jobs queue on the server, the runner claims one at a time, silence means lost

- Status: Accepted
- Issues: #2706, #2701 (decisions 5, 7, 10, 15)
- Affects: `api/internal/platform/factory/` (`internal/services/jobs.go`, `connect_jobs.go`,
  migration `00004`), `proto/factory/v1`, `api/cmd/api/routes.go` (`isLongRequest`)

## Context

Pressing Run on a task has to reach the owner's Mac without the Mac accepting any
inbound connection, survive the Mac being offline, and never let an agent approve
its own work ([ADR-0041](adr-0041-factory-runner-pairing.md)).

## Decision

- **A `jobs` table is the queue.** A human admin's Run adds a row and moves the task to In progress; the oldest queued row is claimed first, one unfinished job per task, and a runner holds at most its `max_jobs`. Stop cancels a job; the runner learns it must stop from its next check-in.
- **The runner pulls.** A claim is a long-held request ([ADR-0017](adr-0017-long-request-handler-deadlines.md)) and a periodic check-in keeps a job alive, so the Mac accepts no inbound connection.
- **The prompt is frozen at Run** from the task and the Run text (the repo's own AGENTS.md carries the rest); agent comments never enter it. A task whose title or body holds unrendered text (an HTML comment, an invisible character) can't be run or moved to Ready, since an agent could have written text the owner never saw.
- **Five minutes of silence is lost**, and the task moves to Needs you. No server timer runs: the sweep runs on a claim and when the board is read.
- **A job runs until its PR merges.** The runner keeps the VM and resumes the same agent session with what failed (a gate, a push or PR problem, a red `ci-pass`), so one running job blocks the queue meanwhile. It stops to Needs you with a reason on a `Blocked:` line, a repeated failure, a closed PR, a stop, shutdown, an attempt cap or a time cap.
- **A usage limit** ends the job as limited; the server queues a continuing job and holds claims for that harness until the reset.
- **Harness and model are the Mac's choice**, not the board's; the job records them.
- **Auto pickup** (per repo, off by default) lets a claiming runner take the oldest highest-priority Ready task. The job is built from the title and body snapshotted at the Ready move, never later edits; a board-wide pause overrides it.
- **Reply** queues a child job that resumes the parent's session.
- **Logs are polled, not pushed**, with secrets removed, capped per job and expiring.
- **Slack** notices go out in the background and never fail the work they report.

## Consequences

- A lost job is only marked when something reads the board or a runner claims;
  a runner that never returns leaves the task In progress until then.
- A job's comment is written as `runner:<id>` or `system:factory`, so the board can
  tell it from a person's or an agent's.
- One job at a time means a job waiting for CI blocks the queue; parallel jobs
  are out of scope here.
- Reading CI needs the GitHub App to hold `checks: read`; without it the runner
  can't see a red `ci-pass`, and the job ends at six hours unmerged.

## Revisit when

Jobs need cancelling (Stop, #2707), the queue needs more than FIFO, or a timer on
the server replaces the read-time sweep.
