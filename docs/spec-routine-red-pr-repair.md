# Spec: red PR repair routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1448, part of epic #1338. Depends on #1439 (merged) and #1446
  (structural template, PR #1684).

## What this is

A claude.ai/code scheduled routine, running once a morning against this
repository, whose prompt runs the `red-pr-repair` skill
(`.claude/skills/red-pr-repair/SKILL.md`). It closes the other half of
#1338's self-healing loop: Renovate already opens dependency-bump PRs and CI
already verifies them, and the `ready-issues-sweep` executor routine
(issue #1447) already turns a Ready-column issue into a PR — what neither of
those covers is a PR that goes red and just sits there with no one watching
it overnight. This routine reads CI state for every currently-red PR
carrying the `dependencies` label or a `claude/`-prefixed branch, and either
fixes it, unsticks it (the Codecov-stall case), or leaves it open with an
explanatory comment — it never merges anything beyond what the existing
auto-merge rule already would, and it never closes/abandons a PR it can't
fix.

This is a spec, not an ADR — there's no alternative design being weighed
here, just a reproducible record of exactly what to paste into the
claude.ai routines UI, per this repo's own convention that an ADR without an
"alternatives considered" section is a spec instead (`docs/TEMPLATE-adr.md`).

## Why this can't be created through the trigger-creation API

Issue #1438 established empirically that the trigger-creation API silently
drops any connector the creating session doesn't itself hold — a routine
created that way never gets the `tools-apps` MCP connector wired up, and
nothing in the API response says so. The routine described here **must be
created by hand** at claude.ai/code/routines, in a session that already
holds both connectors, for the same reason `docs/spec-routine-nightly-
maintenance-sweep.md` (issue #1446) hit the identical constraint. There is
no code or API path that completes this step — it's recorded here purely so
it's reproducible without re-deriving the exact prompt/connector list from
scratch.

## Manual setup steps

1. Go to `claude.ai/code/routines` → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: daily, once a day, at **07:00** local time. This is
   deliberately a morning slot, distinct from the nightly maintenance
   sweep's (issue #1446) and the Ready-column executor's (issue #1447)
   02:00/04:00 slots — this routine's whole point is driving a PR to green
   *before* the day starts, after whatever the other two routines did
   overnight has had a chance to go red. This consumes 1 of the 5 daily
   routine runs available — leave headroom for the remaining slices of epic
   #1338 that also want a scheduled budget.
4. Connectors — **both required**, attached in the same session that
   creates the routine:
   - `tools-apps` — the MCP server at `https://tools.xdoubleu.com/api/apps/mcp`
     (`claude mcp add --transport http tools-apps
     https://tools.xdoubleu.com/api/apps/mcp`, or the equivalent claude.ai
     connector). Supplies `get_failing_pull_requests`/`get_workflow_runs`
     (the read tools this routine pulls CI state from) and `record_action`
     (the run-history open/close wrapper).
   - GitHub — to check out PR branches, push fixing/unsticking commits,
     comment on PRs it can't fix, and read PR/check state directly (`gh pr
     checkout`, `gh run view --log-failed`, `gh pr comment`).
5. Prompt: paste the block below **verbatim** into the routine's prompt
   field.

## Routine prompt (paste verbatim)

```
Run the red-pr-repair skill (.claude/skills/red-pr-repair/SKILL.md). If
Skill({skill: "red-pr-repair"}) errors with "Unknown skill", that means
this session's environment doesn't register repo-local skills (see issue
#1624) — fall back to reading .claude/skills/red-pr-repair/SKILL.md
directly with the Read tool and follow its steps by hand instead of
giving up; each dispatched subagent needs the same fallback. If any
subagent finds it has no add_repo tool or no authenticated GitHub push
access (see issue #1625) and can only reach this public repo via an
unauthenticated read-only clone, it cannot push a fixing/unsticking
commit or comment on the PR — have it report that as the reason it
couldn't complete rather than silently stopping. Call
record_action with mode=open, trigger_source=schedule,
routine_name=red-pr-repair before pulling any data. Pull
get_failing_pull_requests and filter to PRs carrying the dependencies label
or a claude/-prefixed branch that are currently red. For each one,
dispatch an isolated subagent (isolation: worktree) that checks out the
PR's existing branch directly (never start-task — the branch and issue
already exist), diagnoses the failure from the actual failing job's logs,
and does exactly one of: (a) if ci-pass is failing with "Timed out waiting
for Codecov to report," push a trivial non-empty commit to unstick it —
never re-run the check, a new commit is the only documented fix
(CLAUDE.md's CI section); (b) if a concrete fix is determinable (dependency
conflict, lint failure, a test needing an update for the bump's behavior
change), make the fix, verify it locally, and push it; (c) if no fix is
determinable, leave the PR open exactly as-is and post a gh pr comment
explaining what was investigated and why it couldn't be resolved
automatically — never close or abandon the PR. Never merge anything beyond
what finish-task's existing auto-merge rule already allows. Do not ask me
anything and do not wait on a response — decide and act, or leave the
explanatory comment, and move on. Once every dispatched subagent has
reported back, call record_action again with mode=close (using the id the
open call returned), recording outcome as no_action_needed, succeeded, or
failed, and a short summary of what happened per PR.
```

## Known platform constraints (as of 2026-09-16)

Two gaps found by #1438's spike, both still open as #1624 and #1625, bear
directly on this routine — more so than on the nightly sweep, since this
one has to end in a pushed commit or PR comment:

- **#1624** — a routine-fired session may not register repo-local skills
  through the `Skill` tool at all. The prompt above now has an explicit
  read-the-file fallback, both for this routine's own skill and for each
  dispatched subagent. Conflicting evidence: a Claude Code Remote/web
  session working on this repo (not itself a fired routine) does list
  `red-pr-repair` and its sibling skills as invocable — needs
  re-confirming against an actual fired routine.
- **#1625** — a routine-fired session may have no `add_repo` tool and no
  authenticated GitHub push access, only an unauthenticated read-only
  clone (works because this repo is public). **Partially re-confirmed
  2026-09-17**: a subagent dispatched the identical way (`isolation:
  "worktree"`, via the `Agent` tool) by the sibling `ready-issues-sweep`
  routine — while working issue #1625 itself — found a git remote with
  working push credentials already attached and the full `mcp__github__*`
  tool set, no `add_repo` call needed or available. This routine's own
  subagents are dispatched through the same mechanism, so the same result
  is expected here too, but it hasn't been directly observed on a
  `red-pr-repair` dispatch — see #1625 for the full writeup and re-check
  directly the next time this routine actually fires. Until then, treat
  the pessimistic read above as superseded-but-unconfirmed rather than
  still-expected.

## Related

- `.claude/skills/red-pr-repair/SKILL.md` — the skill this routine runs.
- `docs/spec-routine-nightly-maintenance-sweep.md` (issue #1446) — the
  sibling nightly routine and the doc-format template this one mirrors.
- `README.md`'s "Apps MCP server" section — `record_action`/
  `get_automated_actions` (issue #1441), the mechanism that populates
  `global.automated_actions` for every routine run including this one.
- Root `CLAUDE.md`'s CI section — the stuck-Codecov failure mode and its
  documented workaround (a new commit, never a re-run); this doc's own
  originating issue (#1448) names this guidance as living in a
  `docs/spec-ci-pipeline.md` file, which no longer exists after the docs
  consolidation in #1464 — the same content now lives directly in
  `CLAUDE.md`.
