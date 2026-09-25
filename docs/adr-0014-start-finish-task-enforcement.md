# ADR-0014: Enforce the start-task/finish-task pairing with `ExitPlanMode` and `Stop` hooks

- Status: Accepted
- Issues: #1236, #1238, #1400, #1619, #1440, #1706
- Affects: `.claude/settings.json`, `.claude/hooks/stop-check-unshipped-work.sh`, `.claude/skills/start-task/`, `.claude/skills/finish-task/`, root `Makefile` (`hooks/test`)

## Context

14 of 16 sessions that skipped `start-task` also skipped `finish-task` (#1236):
plan approval felt like starting the task, consuming `start-task`'s trigger.
Written rules were read after the point they applied. (Repo-wide
`permissions.defaultMode: plan` was later removed, #1706, because unattended
routines can't be approved out of Plan Mode; the hooks still apply whenever
plan mode is used.)

## Decision

Hooks in `.claude/settings.json`, all exercised by `make hooks/test`:

- **`PostToolUse` on `ExitPlanMode`**: reminds that an approved plan hasn't
  started the task; `start-task` still runs before the first edit.
- **`Stop`** (`stop-check-unshipped-work.sh`): blocks stopping, once per
  commit, in a worktree with unpushed commits or changes and no PR. It checks
  via `gh`, or else the GitHub REST API using the credential
  `git credential fill` resolves — no new secret. If it can't tell, it doesn't
  block (#1400).
- **`PreToolUse` on `Edit`/`Write`/`NotebookEdit`** (#1619): denies writes
  inside the repo but outside the session's active worktree.

## Alternatives considered

- **Documentation alone** — produced the 14-of-16 figure.
- **`Stop` hook calling an MCP tool** — impossible; hooks run outside the
  agent's tool loop (#1440).

## Consequences

- Claude Code on the web has no marketplace plugins or `gh`; the wrapper skills
  document in-repo fallbacks, and the `Stop` hook uses the REST fallback.
- Hook changes must keep `make hooks/test` green (both `gh` and REST paths).

## Revisit when

The harness gains a first-class notion of "task started".
