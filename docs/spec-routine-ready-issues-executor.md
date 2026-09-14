# Spec: nightly ready-issues executor routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1447, part of epic #1338. Depends on #1440, #1446 (both merged).

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
Run the ready-issues-sweep skill (.claude/skills/ready-issues-sweep/SKILL.md)
in its unattended mode. Pull the "Ready" column of the project board,
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
