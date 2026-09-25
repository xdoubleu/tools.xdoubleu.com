# Spec: nightly ready-issues executor routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1447, part of epic #1338. Depends on #1440, #1446. Bug-only scope
  from #1744.

A nightly claude.ai/code routine running the `ready-issues-sweep` skill
(`.claude/skills/ready-issues-sweep/SKILL.md`) unattended: every
**`bug`-labeled** issue in the board's "Ready" column is driven by a subagent
through `start-task` → fix → `finish-task` to an open PR (auto-merge as
`finish-task` decides). Non-`bug` issues are skipped — unattended feature work
is a different risk than fixing a diagnosed bug.

**Create by hand** at claude.ai/code/routines, in a session holding every
connector below — the trigger-creation API silently drops connectors the
creating session lacks (#1438). The live routine's prompt can only be edited
by its owner in that UI (`update_trigger` is refused for it), so paste this
doc's prompt there whenever it changes.

**Moving to GitHub Actions:** `.github/workflows/routine-ready-issues-executor.yml`
([spec-agent-routines-on-actions](spec-agent-routines-on-actions.md)) carries
this prompt, adapted to OpenCode. To switch over, disable the claude.ai routine
and set `ROUTINE_READY_ISSUES_EXECUTOR_ENABLED=true`.

## Setup

1. claude.ai/code/routines → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: nightly at **04:00** — after the 02:00 maintenance sweep, so
   tonight's newly filed issues are picked up the same night (1 of the 5
   daily routine runs).
4. Connectors (both required):
   - `tools-apps` — `https://tools.xdoubleu.com/api/apps/mcp` (`claude mcp
     add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp`
     or the claude.ai connector). Supplies `record_action`.
   - GitHub — read the Ready column (`get_project_issues_by_status`); branches,
     issues, PRs, and CI for every subagent.
5. Paste the prompt below verbatim.

## Routine prompt (paste verbatim)

```
Run the ready-issues-sweep skill (.claude/skills/ready-issues-sweep/SKILL.md)
in its unattended mode. If Skill({skill: "ready-issues-sweep"}) errors with
"Unknown skill", read .claude/skills/ready-issues-sweep/SKILL.md with the
Read tool and follow its steps by hand instead of giving up; each dispatched
subagent needs the same fallback for start-task/finish-task. If a subagent
has no authenticated GitHub push access (only a read-only clone), it must
report that as why it couldn't complete its issue rather than silently
stopping. Pull the "Ready" column; skip every issue not labeled "bug" per
.claude/github-triage.config.json's labels.types, noting each skipped issue
and its label in the final summary. Dispatch one isolated subagent per
remaining issue and let each drive it through start-task and finish-task to
an open PR; do not override finish-task's auto-merge decision either way.
Do not ask me anything and do not wait on a response — decide and act, or
report why an issue couldn't reach a PR (e.g. the skill's empty-body
exception or the read:project scope check), and move on. Call record_action
with mode=open, trigger_source=schedule, routine_name=ready-issues-executor
before pulling the Ready column, and with mode=close (the id the open call
returned) once every subagent has reported back, recording outcome
no_action_needed, succeeded, or failed and a short summary.
```

## Known platform constraints

- **#1624** — a fired session may not register repo-local skills; the
  prompt's read-the-file fallback covers the routine and its subagents.
- **#1625** — a fired session may lack authenticated push access. Observed
  2026-09-17 on one dispatched subagent: push credentials and the full
  `mcp__github__*` set were present (`git push --delete` got a 403, push
  worked). One data point, not a guarantee — the prompt's report-don't-stop
  fallback still applies.

## Related

- `docs/spec-routine-nightly-maintenance-sweep.md` — the detection half this
  routine executes.
- `README.md`'s "Apps MCP server" — `record_action`/`get_automated_actions`.
