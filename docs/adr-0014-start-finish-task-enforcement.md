# ADR-0014: Enforce the start-task/finish-task pairing with `ExitPlanMode` and `Stop` hooks

- Status: Accepted
- Issues: #1236, #1238, #1400
- Affects: `.claude/settings.json`, `.claude/skills/start-task/`, `.claude/skills/finish-task/`, `api/Makefile` (`hooks/test`)

## Context

Sessions kept skipping the workflow, and the two halves failed together: **14 of
16 sessions that skipped `start-task` also never ran `finish-task`** — because
`finish-task` is reached via `start-task`'s hand-off, not independently (#1236).

The specific trigger-stealer: `permissions.defaultMode` is `plan`, so nearly
every session here begins by planning, and **plan approval consumes
`start-task`'s trigger**. An approved plan feels like the task has started; the
whole start/finish pairing is skipped with it.

## Decision

Two hooks in `.claude/settings.json`:

- A **`PostToolUse` hook on `ExitPlanMode`** states that exiting plan mode does
  not count as having started the task — at the moment it matters. `start-task`
  still runs before the first edit, with the approved plan recorded in the
  tracking issue's `## Plan` section.
- A **`Stop` hook** blocks stopping in a worktree that has commits ahead of
  `origin/main` (or uncommitted changes) and no PR, once per commit.

Both are exercised by `make hooks/test` (#1238).

## Alternatives considered

### Documentation alone

That was the status quo, and the 14-of-16 figure is what it achieved. The rules
were already written down; they were being read after the point where they
applied.

### Blocking harder in web sessions

Not possible — see below.

## Consequences

- **A Claude Code on the web session has neither the marketplace plugins nor
  `gh`** (GitHub access is via the `github` MCP tools). Both wrapper skills carry
  a "When a delegated skill isn't installed" section spelling out the in-repo
  fallback, and the `Stop` hook — unable to confirm a PR exists without `gh` —
  treats "can't tell" as "don't block" rather than false-firing (#1400).

  **So nothing external forces the PR to be opened in a web session. Doing it
  unprompted is the only guardrail left there.**
- Hook changes need `make hooks/test` to stay green.

## Revisit when

The harness offers a first-class notion of "task started", making the
plan-approval trigger-stealing problem structural rather than behavioral.
