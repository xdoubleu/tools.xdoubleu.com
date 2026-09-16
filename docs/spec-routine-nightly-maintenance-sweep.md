# Spec: nightly maintenance sweep routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1446, part of epic #1338. Depends on #1439, #1441 (both merged).

## What this is

A claude.ai/code scheduled routine, running once a night against this
repository, whose prompt runs the `monitoring-sweep` skill
(`.claude/skills/monitoring-sweep/SKILL.md`) in its **unattended** mode:
every real problem the sweep finds — Sentry errors, red CI/failing PRs,
Grafana-managed alert state (including the CI/dependency-PR signals that
now live there instead of in a removed app-side notifier job), security
alerts, orphaned storage — becomes a refined GitHub tracking issue on the
board. It never opens a PR or writes a code fix itself; that's a later,
separate slice (issue #1447) that turns a refined issue into a PR the same
way `ready-issues-sweep` does. This keeps a bad or over-eager sweep from
turning directly into a bad PR with no human review before it lands.

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
holds both connectors, for the same reason #1444's webhook path and this
issue's own scope note both point back to #1438. There is no code or API
path that completes this step — it's recorded here purely so it's
reproducible without re-deriving the exact prompt/connector list from
scratch.

## Manual setup steps

1. Go to `claude.ai/code/routines` → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: nightly, once a day, at an off-peak hour (this consumes 1 of
   the 5 daily routine runs available — leave headroom for the other
   slices of epic #1338 that also want a nightly/scheduled budget).
4. Connectors — **both required**, attached in the same session that
   creates the routine:
   - `tools-apps` — the MCP server at `https://tools.xdoubleu.com/api/apps/mcp`
     (`claude mcp add --transport http tools-apps
     https://tools.xdoubleu.com/api/apps/mcp`, or the equivalent
     claude.ai connector). Supplies every read tool the sweep pulls data
     from (`get_sentry_issues`, `get_failing_pull_requests`,
     `get_workflow_runs`, `get_security_alerts`, `get_storage_stats`,
     `get_slow_transactions`, `get_grafana_alerts`, `prom_query`) plus the
     mutating tools it's allowed to call (`record_action` always;
     `resolve_sentry_issue`/`dismiss_security_alert` only for a confirmed
     duplicate/false positive).
   - GitHub — to file/update tracking issues via `refine-issue`/`gh
     issue create` and to read/dispatch against the project board.
5. Prompt: paste the block below **verbatim** into the routine's prompt
   field.

## Routine prompt (paste verbatim)

```
Run the monitoring-sweep skill (.claude/skills/monitoring-sweep/SKILL.md).
If Skill({skill: "monitoring-sweep"}) errors with "Unknown skill", that
means this session's environment doesn't register repo-local skills (see
issue #1624) — fall back to reading .claude/skills/monitoring-sweep/
SKILL.md directly with the Read tool and follow its steps by hand instead
of giving up. Run it in its unattended/detection-only mode. Pull every
monitoring data source
per the skill's step 1 — including get_grafana_alerts, which is now the
ground truth for CI-red and dependency-PR alert state as well as Sentry
and security-alert state — cluster into independent workstreams, and
dispatch one subagent per workstream. Each subagent should root-cause its
problem and file or update one refined GitHub tracking issue describing
the root cause and a concrete suggested fix — never write a code fix,
never open a PR, never create a branch. When a signal can't be judged for
lack of context, or looks like a false positive, file or update an issue
proposing a concrete infra/grafana/provisioning/alerting/rules.yml change
(threshold, added label, richer annotation) instead of silently skipping
it. Do not ask me anything and do not wait on a response — decide and
act, or file the issue documenting why you couldn't, and move on. Call
record_action with mode=open, trigger_source=schedule,
routine_name=nightly-maintenance-sweep before pulling any data, and call
it again with mode=close (using the id the open call returned) once every
workstream has reported back, recording outcome as no_action_needed,
succeeded, or failed and a short summary of what happened.
```

## Known platform constraints (as of 2026-09-16)

Two gaps found by #1438's spike were tracked as #1624 and #1625.

- **#1624** — a routine-fired Claude Code on the web session may not
  register repo-local skills through the `Skill` tool at all. The prompt
  above now has an explicit read-the-file fallback for this. Conflicting
  evidence: a Claude Code Remote/web session working on this repo (not
  itself a fired routine) does list `monitoring-sweep` and its sibling
  skills as invocable — this needs re-confirming against an actual fired
  routine before #1624 can close.
- **#1625 — resolved for the shared dispatch path, confirmed 2026-09-16.**
  This routine only ever files/updates tracking issues via GitHub and
  never pushes code, so it was always less exposed than #1447/#1448 — but
  the underlying question (does a routine-fired subagent have an
  authenticated GitHub connector at all, not just a read-only clone) is
  now answered: a worktree subagent dispatched by an actual
  `trigger_source=schedule` routine firing was empirically found to have
  authenticated `mcp__github__*` tools (including issue read/write) and a
  pre-cloned git worktree, no `add_repo` call needed. That confirmation
  ran inside `ready-issues-executor`, not this routine, but both dispatch
  through the identical mechanism (a scheduled routine session in the
  Claude Code CLI/Agent-SDK harness fanning out `Agent`-tool `isolation:
  "worktree"` subagents), so the same access should apply here too — see
  `docs/spec-routine-ready-issues-executor.md`'s "Known platform
  constraints" section for the full writeup, and #1625's closing comment.
  The bare "Claude Code on the web" runtime #1438's spike tested remains a
  distinct, more constrained product surface that this finding does not
  speak to.

## Related

- `.claude/skills/monitoring-sweep/SKILL.md` — the skill this routine runs,
  including the interactive-vs-unattended mode split and the
  self-correcting `rules.yml`-change instruction.
- `README.md`'s "Apps MCP server" section — `record_action`/
  `get_automated_actions` (issue #1441) and `api/internal/routines` (issue
  #1444), the two distinct paths that populate `global.automated_actions`.
- `docs/convention-mcp-gap-first.md` — the same fix-the-gap-first principle
  this routine's self-correcting instruction applies to alert-rule quality
  instead of MCP tool coverage.
- `docs/adr-0022-prometheus-grafana-metrics.md` — why alerting lives in
  Grafana and `get_grafana_alerts` is the read path Prometheus `ALERTS{}`
  cannot answer.
