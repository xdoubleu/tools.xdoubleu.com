# Spec: nightly maintenance sweep routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1446, part of epic #1338. Depends on #1439, #1441.

A nightly claude.ai/code routine running the `monitoring-sweep` skill
(`.claude/skills/monitoring-sweep/SKILL.md`) in **unattended, detection-only**
mode: every problem becomes a refined `bug` issue; the only direct mutations
are `record_action` and resolving confirmed-fixed/duplicate Sentry issues
(`resolve_sentry_issue`) or false-positive alerts. `ready-issues-executor`
turns the issues into PRs later.

**Create by hand** at claude.ai/code/routines, in a session holding every
connector below — the trigger-creation API silently drops connectors the
creating session lacks (#1438).

**Moving to GitHub Actions:** `.github/workflows/routine-nightly-maintenance-sweep.yml`
([spec-agent-routines-on-actions](spec-agent-routines-on-actions.md)) carries
this prompt, adapted to OpenCode. To switch over, disable the claude.ai routine
and set `ROUTINE_NIGHTLY_MAINTENANCE_SWEEP_ENABLED=true`.

## Setup

1. claude.ai/code/routines → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: nightly at **02:00** (1 of the 5 daily routine runs).
4. Connectors (both required):
   - `tools-apps` — `https://tools.xdoubleu.com/api/apps/mcp` (`claude mcp
     add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp`
     or the claude.ai connector). Read tools plus `record_action`,
     `resolve_sentry_issue`, `dismiss_security_alert`.
   - GitHub — file/update issues, read the project board.
5. Paste the prompt below verbatim.

## Routine prompt (paste verbatim)

```
Run the monitoring-sweep skill (.claude/skills/monitoring-sweep/SKILL.md) in
its unattended/detection-only mode. If Skill({skill: "monitoring-sweep"})
errors with "Unknown skill", read .claude/skills/monitoring-sweep/SKILL.md
with the Read tool and follow its steps by hand instead of giving up.
Follow every step, including: get_grafana_alerts first; step 2's Sentry
resolution backstop inline (no subagent), resolving only with evidence the
fix shipped and the error stopped; one subagent per workstream that files or
updates one refined GitHub issue with root cause and a concrete suggested
fix, always labeled `bug` — never a code fix, branch, or PR; and a
rules.yml-change issue for any signal that lacks context or is a false
positive. Do not ask me anything and do not wait on a response — decide and
act, or file an issue documenting why you couldn't, and move on. Call
record_action with mode=open, trigger_source=schedule,
routine_name=nightly-maintenance-sweep before pulling any data, and with
mode=close (the id the open call returned) once every workstream has
reported back, recording outcome no_action_needed, succeeded, or failed and
a short summary.
```

## Known platform constraints

- **#1624** — a fired session may not register repo-local skills; the
  prompt's read-the-file fallback covers it.
- **#1625** — a fired session may lack authenticated GitHub access. Issue
  filing needs the GitHub connector; a sibling routine's subagent had full
  `mcp__github__*` write access (2026-09-17), not yet observed on this
  routine's own path.

## Related

- `docs/adr-0022-prometheus-grafana-metrics.md` — why `get_grafana_alerts`,
  not Prometheus `ALERTS{}`, is the alert read path.
- `README.md`'s "Apps MCP server" — `record_action`/`get_automated_actions`.
