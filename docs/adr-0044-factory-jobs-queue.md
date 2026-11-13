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
  task's title or body holds text that isn't rendered (an HTML comment, an invisible
  character), Run and the Ready move are refused: an agent could have written text
  the owner never saw. Who last edited the body doesn't matter, so an agent-refined
  task runs once the owner moves it to Ready.
- **Five minutes of silence is lost.** A running job whose last check-in is that
  old becomes `lost` and its task moves to Needs you. No timer runs on the
  server; the sweep runs when a runner claims a job and when the board is read,
  so a dead runner is noticed the next time the owner looks.
- **A job runs until its PR merges.** The runner keeps the job's VM and resumes
  the same agent session with what failed: a gate or host check (`Not published:`),
  a push or PR problem, or a red `ci-pass` after the PR opens (read with the push
  token; the failing checks' names and annotations go back to the agent). The job
  stays `running` and checks in throughout, so the lost sweep still works; one
  running job at a time means the queue waits meanwhile. It ends on a merge, then
  the poller moves the task to Testing. Stops, each to Needs you with the reason:
  a `Blocked:` line, the same failure twice with no new commit, a closed PR, a
  stop from the board, a runner shutdown, 20 attempts, or six hours unmerged. A
  usage limit waits in the VM until its reset (an hour if unsaid), then resumes.
  The timeout is per attempt. Each attempt adds a log line and a task comment
  (`AddJobComment`, running jobs only). A gate marked `not run (CI checks it)`,
  for code the sandbox can't build, doesn't block the PR.
- **A job that ends without a merge moves its task to Needs you** with the outcome
  as a comment; one that opened a PR keeps its link.
- **A job the Claude usage limit stopped** is `failed` with `limited` and, if the
  CLI said, `limit_resets_at` (kept only within the next 15 days). The runner reads
  it from the stream's `rate_limit_event` (status `rejected`) or an error result;
  those shapes come from the CLI's own strings, not a capture of a real hit. The
  board shows the latest limit in force as "Limited until ...", an hour after the
  job ended when the CLI gave no time. While one is in force auto pickup leaves a
  Ready task that would run on Claude Code waiting, instead of failing it too.
- **The log is polled, not pushed.** The runner forwards the harness's stream-json
  lines every 2 seconds, secrets removed, to `AppendJobEvents`; `ListJobEvents`
  serves them to a human admin in a browser. A job keeps at most 5000 events, for
  30 days.
- The branch is `agent/t<number>-<slug>` (the task number, shown as T-<number>), chosen by the server and checked again
  by the runner.

- **Reply** queues a child job (`parent_job_id`) with the reply as its prompt, the
  parent's harness and branch. The runner receives the parent's session id as
  `resume_session_id`. It needs a finished newest job that left a session, and a
  task that isn't Done.

- **Slack** (`FACTORY_SLACK_WEBHOOK_URL`, https only; unset, nothing is posted) gets a
  message when a job opens a PR, fails, finishes without one, or is lost, when a
  runner pairs or an agent account is created, and once when a job has waited ten
  minutes with no runner online. A message carries the task title as a link to `/factory?task=<id>`
  and the outcome's first line, escaped so a title can't mention the channel. It is
  sent in the background and never fails the work it reports.

- **Harness and model** are frozen on each job. A repo has a default harness and, for
  OpenCode, a default OpenRouter model (`vendor/model`; Claude Code chooses its own
  and takes none; the runner runs `opencode run --format json --auto` with the prompt
  read from stdin by a shell step). A Run can pick a harness and model for that job alone, and a
  Reply keeps the job it continues, because a session belongs to its harness.

- **Auto pickup** (`repos.auto_pickup`, off by default) lets a claiming runner take the
  oldest highest-priority Ready task when nothing is queued or running. The Ready
  move snapshots the task's title and body (`ready_title`, `ready_body`); the job
  is built from that, never from later edits. A board-wide pause switch overrides
  every repo.

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
