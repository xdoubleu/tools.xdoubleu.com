# Spec: red PR repair routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1448, part of epic #1338. Depends on #1439, #1446. CI-fired red
  `main` trigger from #1722.

A daily claude.ai/code routine running the `red-pr-repair` skill
(`.claude/skills/red-pr-repair/SKILL.md`): every red PR labelled
`dependencies` or on a `claude/` branch is fixed, unstuck (Codecov stall), or
left open with an explanatory comment — never merged beyond the existing
auto-merge rule, never closed.

**Create by hand** at claude.ai/code/routines, in a session holding every
connector below — the trigger-creation API silently drops connectors the
creating session lacks (#1438).

## Second trigger: a red `main`

`main.yml`'s `notify-main-ci-red` job POSTs a Grafana-alert-shaped body
(`labels.routine: "red-pr-repair"`) to `/webhooks/grafana-alert`
(`api/cmd/api/routines_webhook.go`) the moment a push-to-main job fails,
authenticated with the `ROUTINE_FIRE_TOKEN` bearer (also a GitHub Actions
secret, see `infra/README.md`). It is the same route Grafana's `routine-fire`
contact point (`infra/grafana/provisioning/alerting/contactpoints.yml`)
targets, skipping Grafana's evaluation latency. The fix still goes through a
new PR — see the skill's "Entry point 2".

## Setup

1. claude.ai/code/routines → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: daily at **07:00** local — after the 02:00/04:00 routines, so
   their overnight PRs are green before the day starts (1 of the 5 daily
   routine runs).
4. Connectors (both required):
   - `tools-apps` — `https://tools.xdoubleu.com/api/apps/mcp` (`claude mcp
     add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp`
     or the claude.ai connector). Supplies `get_failing_pull_requests`,
     `get_workflow_runs`, `record_action`.
   - GitHub — check out PR branches, push, comment, read checks (`gh pr
     checkout`, `gh run view --log-failed`, `gh pr comment`).
5. Paste the prompt below verbatim.

## Routine prompt (paste verbatim)

```
Run the red-pr-repair skill (.claude/skills/red-pr-repair/SKILL.md). If
Skill({skill: "red-pr-repair"}) errors with "Unknown skill", read
.claude/skills/red-pr-repair/SKILL.md with the Read tool and follow its
steps by hand instead of giving up; each dispatched subagent needs the same
fallback. If a subagent has no authenticated GitHub push access (only a
read-only clone), it must report that as why it couldn't complete rather
than silently stopping. Call record_action with mode=open,
trigger_source=schedule, routine_name=red-pr-repair before pulling any data.
Pull get_failing_pull_requests and filter to red PRs carrying the
dependencies label or a claude/-prefixed branch. For each, dispatch an
isolated subagent (isolation: worktree) that checks out the PR's existing
branch (never start-task), diagnoses from the failing job's logs, and does
exactly one of: (a) ci-pass "Timed out waiting for Codecov to report" → push
a trivial non-empty commit, never re-run the check; (b) a determinable fix →
fix, verify locally (including after any merge-conflict resolution), push;
(c) otherwise leave the PR open as-is and post a gh pr comment explaining
what was investigated and why — never close or abandon it. Never merge
beyond finish-task's existing auto-merge rule. Do not ask me anything and do
not wait on a response — decide and act, or leave the comment, and move on.
Once every subagent has reported back and every push has landed, call
record_action with mode=close (the id the open call returned), recording
outcome no_action_needed, succeeded, or failed and a short per-PR summary.
```

## Known platform constraints

- **#1624** — a fired session may not register repo-local skills; the
  prompt's read-the-file fallback covers the routine and its subagents.
- **#1625** — a fired session may lack authenticated push access. A
  sibling routine's identically dispatched subagent had push credentials and
  the full `mcp__github__*` set (2026-09-17); unconfirmed on this routine —
  re-check next time it fires.

## Related

- `AGENTS.md`'s CI section — the Codecov-stall workaround.
- `README.md`'s "Apps MCP server" — `record_action`/`get_automated_actions`.
