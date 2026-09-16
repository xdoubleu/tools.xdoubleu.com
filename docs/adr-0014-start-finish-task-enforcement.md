# ADR-0014: Enforce the start-task/finish-task pairing with `ExitPlanMode` and `Stop` hooks

- Status: Accepted
- Issues: #1236, #1238, #1400, #1619, #1440, #1706
- Affects: `.claude/settings.json`, `.claude/hooks/stop-check-unshipped-work.sh`, `.claude/skills/start-task/`, `.claude/skills/finish-task/`, root `Makefile` (`hooks/test`)

## Context

Sessions kept skipping the workflow, and the two halves failed together: **14 of
16 sessions that skipped `start-task` also never ran `finish-task`** — because
`finish-task` is reached via `start-task`'s hand-off, not independently (#1236).

The specific trigger-stealer, at the time: `.claude/settings.json` set
`permissions.defaultMode: "plan"` repo-wide, so nearly every session here began
by planning, and **plan approval consumed `start-task`'s trigger**. An approved
plan feels like the task has started; the whole start/finish pairing was
skipped with it. (`permissions.defaultMode: plan` was later removed — #1706 —
once epic #1338's unattended self-healing routines started actually hitting
this repo, and Plan Mode has no way to be approved out of without a human
present. The three hooks below don't depend on that default, though — they
fire whenever a session enters plan mode at all, by explicit request or
otherwise, so they stayed exactly as necessary after the default's removal as
they were before it.)

## Decision

Two hooks in `.claude/settings.json`:

- A **`PostToolUse` hook on `ExitPlanMode`** states that exiting plan mode does
  not count as having started the task — at the moment it matters. `start-task`
  still runs before the first edit, with the approved plan recorded in the
  tracking issue's `## Plan` section.
- A **`Stop` hook** (`.claude/hooks/stop-check-unshipped-work.sh`) blocks
  stopping in a worktree that has commits ahead of `origin/main` (or
  uncommitted changes) and no PR, once per commit. It checks for an existing
  PR via `gh` when available, and via a direct GitHub REST API call
  otherwise — authenticated with whatever credential `git credential fill`
  resolves for the repo's configured credential helper, the same credential
  that already has to exist for the branch to have been pushed in the first
  place. This introduces no new env var or deploy secret. If neither path
  can determine an answer (e.g. no `curl`, or no credential helper
  configured), "can't tell" still means "don't block" (#1400) rather than
  false-firing.
- A **`PreToolUse` hook on `Edit`/`Write`/`NotebookEdit`** (#1619) denies a
  write whose target path falls under the repo but outside the session's own
  active worktree — the main checkout or a different worktree of the same
  repo. This is the same rule root `CLAUDE.md`'s "Starting a Task" section
  already states ("never edit in the main checkout or reuse an existing
  branch/worktree") and `task-worktree`'s own skill file already warns about
  ("a stale prefix silently edits the wrong checkout... `pwd` alone won't
  catch it") — turned into something the harness enforces at the moment it
  matters, rather than something only caught after the fact by a diligent
  `git status`.

All three are exercised by `make hooks/test` (#1238, #1619).

## Alternatives considered

### Documentation alone

That was the status quo, and the 14-of-16 figure is what it achieved. The rules
were already written down; they were being read after the point where they
applied.

### Having the `Stop` hook call an MCP tool directly when `gh` is missing

Not possible — a hook is a plain script invoked outside any LLM context, so it
has no access to `mcp__github__*` tools (those exist only inside the agent's
own tool-call loop). A direct GitHub REST API call, authenticated via
whatever credential git itself already resolves, is the mechanism a bash
script actually has available (#1440).

## Consequences

- **A Claude Code on the web session has neither the marketplace plugins nor
  `gh`** (GitHub access for the *agent* is via the `github` MCP tools). Both
  wrapper skills carry a "When a delegated skill isn't installed" section
  spelling out the in-repo fallback. The `Stop` hook itself, being a bash
  script rather than an LLM context, can't call an MCP tool — instead it
  falls back to a direct GitHub REST API call (see Decision) when `gh` is
  absent, so the guardrail holds in both environments (#1440). Only the
  genuine "can't tell" case (no `curl`, or no resolvable credential) still
  means "don't block" rather than false-firing.
- Hook changes need `make hooks/test` to stay green — it exercises both the
  `gh` path and the no-`gh` REST-API fallback.

## Revisit when

The harness offers a first-class notion of "task started", making the
plan-approval trigger-stealing problem structural rather than behavioral.
