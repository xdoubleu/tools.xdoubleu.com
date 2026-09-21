# Spec: nightly ready-issues executor routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1447, part of epic #1338. Depends on #1440, #1446 (both merged).
  #1744 restricted this routine's scope to `bug`-labeled issues only — see
  "Bug-only scope" below. The live routine's own prompt text still needs to
  be updated by hand to match (see that section).

## What this is

A claude.ai/code scheduled routine, running once a night against this
repository, whose prompt runs the `ready-issues-sweep` skill
(`.claude/skills/ready-issues-sweep/SKILL.md`) in its **unattended** mode:
every issue currently sitting in the "Ready" column of the project board
gets driven, by a dispatched subagent, all the way through `start-task` →
fix → `finish-task` to an open PR. Auto-merge behavior for that PR is
exactly what `finish-task` already decides — unchanged by this routine.
This is the execution half of the loop the `monitoring-sweep` routine
(issue #1446) starts: that routine only ever files a refined tracking
issue, never a PR, so nothing in the platform can drive straight from an
automated, possibly-wrong reading of a signal to merged code with no human
in the loop at any point — an issue has to land in Ready (a human or a
prior triage step's judgment call) before this routine will touch it.

This is a spec, not an ADR — there's no alternative design being weighed
here, just a reproducible record of exactly what to paste into the
claude.ai routines UI, per this repo's own convention that an ADR without
an "alternatives considered" section is a spec instead
(`docs/TEMPLATE-adr.md`).

## Bug-only scope

Issue #1744: the routine's first real run picked up Ready-column issues
indiscriminately, including issues that weren't bug fixes. An unattended
routine implementing a brand-new feature with nobody in the loop at any
point is a materially different risk than it fixing an already-diagnosed
bug. The `ready-issues-sweep` skill's step 1 now checks each Ready-column
issue's type label against `.claude/github-triage.config.json`'s
`labels.types` taxonomy before dispatching a subagent, and skips anything
not labeled `bug`, noting the skip in the run's final summary. The routine
prompt below reflects this explicitly. The live routine's own prompt field
can only be edited by hand at claude.ai/code/routines/&lt;id&gt; — an
`update_trigger` call to edit it directly was refused ("this routine was
created via 'http_api', not by an agent... only you can edit it"), so
whoever owns the routine still needs to paste the updated prompt block
below into it to match this doc; this change alone doesn't update the live
routine.

## Why this can't be created through the trigger-creation API

Issue #1438 established empirically that the trigger-creation API silently
drops any connector the creating session doesn't itself hold — a routine
created that way never gets the `tools-apps` MCP connector wired up, and
nothing in the API response says so. The routine described here **must be
created by hand** at claude.ai/code/routines, in a session that already
holds both connectors, for the same reason #1446's own spec
(`docs/spec-routine-nightly-maintenance-sweep.md`) and this issue's own
scope note both point back to #1438. There is no code or API path that
completes this step — it's recorded here purely so it's reproducible
without re-deriving the exact prompt/connector list from scratch.

## Why offset from the maintenance sweep, not run alongside it

Running this routine after the nightly maintenance sweep on the same night
means a problem the sweep detects at 02:00 (filed as a refined Ready-column
issue) can be picked up and turned into a PR at 04:00, the same night.
Running both routines at the same time would have this routine executing
against the board as it stood *before* that night's sweep ran — yesterday's
Ready column, not tonight's.

## Manual setup steps

1. Go to `claude.ai/code/routines` → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: nightly, once a day, at **04:00** — two hours after the
   `nightly-maintenance-sweep` routine's own 02:00 slot (`docs/spec-routine-
   nightly-maintenance-sweep.md`), so a problem detected at 02:00 has a
   settled Ready-column issue to work from by the time this routine runs.
   This consumes 1 of the 5 daily routine runs available — leave headroom
   for the other slices of epic #1338 that also want a nightly/scheduled
   budget.
4. Connectors — **both required**, attached in the same session that
   creates the routine:
   - `tools-apps` — the MCP server at `https://tools.xdoubleu.com/api/apps/mcp`
     (`claude mcp add --transport http tools-apps
     https://tools.xdoubleu.com/api/apps/mcp`, or the equivalent
     claude.ai connector). Supplies `record_action` (open/close the run's
     `automated_actions` row) and `get_automated_actions` (history).
   - GitHub — to read the "Ready" column (`get_project_issues_by_status`),
     and for every subagent's `start-task`/`finish-task` pass: creating
     branches, refining/adopting tracking issues, opening PRs, and watching
     CI.
5. Prompt: paste the block below **verbatim** into the routine's prompt
   field.

## Routine prompt (paste verbatim)

```
Run the ready-issues-sweep skill (.claude/skills/ready-issues-sweep/SKILL.md).
If Skill({skill: "ready-issues-sweep"}) errors with "Unknown skill", that
means this session's environment doesn't register repo-local skills (see
issue #1624) — fall back to reading .claude/skills/ready-issues-sweep/
SKILL.md directly with the Read tool and follow its steps by hand instead
of giving up; each dispatched subagent needs the same fallback for
start-task/finish-task if those are also unresolvable as skills. If any
subagent finds it has no add_repo tool or no authenticated GitHub push
access (see issue #1625) and can only reach this public repo via an
unauthenticated read-only clone, it cannot push a branch or open a PR —
have it report that as the reason it couldn't complete its issue rather
than silently stopping. Run it in its unattended mode. Pull the "Ready"
column of the project board. Before dispatching, check each issue's type
label against .claude/github-triage.config.json's labels.types taxonomy and
skip anything not labeled "bug" — note each skipped issue and its actual
label in the final summary. For the rest,
dispatch one isolated subagent per issue exactly as the skill describes,
and let each subagent drive its issue all the way through start-task and
finish-task to an open PR. Auto-merge behavior is whatever finish-task
already decides for that PR — do not override it either way. Do not ask me
anything and do not wait on a response — decide and act, or (per the
skill's own step 3 exception for an empty-body issue, and step 1's
read:project-scope check) report why a given issue couldn't be driven to a
PR this run, and move on to the rest of the column. Call record_action
with mode=open, trigger_source=schedule, routine_name=ready-issues-executor
before pulling the Ready column, and call it again with mode=close (using
the id the open call returned) once every dispatched subagent has reported
back, recording outcome as no_action_needed, succeeded, or failed and a
short summary of what happened.
```

## Known platform constraints (as of 2026-09-16)

Two gaps found by #1438's spike, both still open as #1624 and #1625, bear
directly on this routine — more so than on the nightly sweep, since this
one has to end in a pushed branch and PR:

- **#1624** — a routine-fired session may not register repo-local skills
  through the `Skill` tool at all. The prompt above now has an explicit
  read-the-file fallback, both for this routine's own skill and for each
  dispatched subagent's `start-task`/`finish-task`. Conflicting evidence:
  a Claude Code Remote/web session working on this repo (not itself a
  fired routine) does list `ready-issues-sweep` and its sibling skills as
  invocable — needs re-confirming against an actual fired routine.
- **#1625** — a routine-fired session may have no `add_repo` tool and no
  authenticated GitHub push access, only an unauthenticated read-only
  clone (works because this repo is public). **Re-confirmed 2026-09-17
  against a real dispatched subagent** — this skill's own step 3, working
  issue #1625 itself as part of a `ready-issues-sweep` run: the subagent's
  worktree had a git remote with working push credentials already attached
  (a throwaway branch pushed cleanly; oddly, deleting that same branch with
  `git push --delete` got a `403` — push and delete aren't symmetric, but
  push is what this routine actually needs) plus the full `mcp__github__*`
  tool set (`create_pull_request`, `create_branch`,
  `create_or_update_file`, …). No `add_repo` tool exists in this
  environment or was needed — access was already there. This is one data
  point from one dispatch, not a platform guarantee, but it means the
  constraint this section used to warn about did not reproduce at the
  layer that actually pushes and opens PRs. See #1625 for the full
  writeup. A future subagent that does hit the no-access case should still
  report it explicitly per the routine prompt's own fallback instruction
  rather than assume this note still holds.

## Related

- `.claude/skills/ready-issues-sweep/SKILL.md` — the skill this routine
  runs, including the interactive-vs-unattended mode split and the
  `record_action` open/close wrapping.
- `docs/spec-routine-nightly-maintenance-sweep.md` — the earlier,
  detection-only routine (issue #1446) this one is offset from; together
  they form the full nightly detect-then-fix loop.
- `README.md`'s "Apps MCP server" section — `record_action`/
  `get_automated_actions` (issue #1441), the tool this routine's step 0/
  step 7 use to make an unattended run observable.
- Issue #1443 — a permanently-open `automated_actions` row (a run that
  opened but never closed) is itself a detectable monitoring problem; the
  same reasoning `monitoring-sweep`'s spec records for its own routine
  applies here.
