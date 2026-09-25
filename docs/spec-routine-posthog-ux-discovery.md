# Spec: weekly PostHog-insights-to-issues routine

- Status: code-side done; the routine itself needs manual setup (see below)
- Issues: #1639, part of epic #1637. Depends on #1638.

A weekly claude.ai/code routine running the `posthog-ux-discovery` skill
(`.claude/skills/posthog-ux-discovery/SKILL.md`): it files proposed UX issues
from PostHog friction signals and never changes UI, branches, or opens PRs.
Until the PostHog SDK (#1638) has a full week of production traffic, every
run is expected to report `no_action_needed`.

**Create by hand** at claude.ai/code/routines, in a session holding all three
connectors below — the trigger-creation API silently drops connectors the
creating session lacks (#1438).

**Moving to GitHub Actions:** `.github/workflows/routine-posthog-ux-discovery.yml`
([spec-agent-routines-on-actions](spec-agent-routines-on-actions.md)) carries
this prompt, adapted to OpenCode. To switch over, disable the claude.ai routine
and set `ROUTINE_POSTHOG_UX_DISCOVERY_ENABLED=true`.

## Setup

1. claude.ai/code/routines → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: weekly, one fixed day, with headroom against the other routines
   (1 of the 5 daily routine runs that day).
4. Connectors (all three required):
   - `tools-apps` — `https://tools.xdoubleu.com/api/apps/mcp` (`claude mcp
     add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp`
     or the claude.ai connector). Supplies `record_action`.
   - GitHub — file/update issues.
   - PostHog's official MCP server — take the URL and auth from PostHog's
     current docs; needs read access to the PostHog Cloud (EU) project.
5. Paste the prompt below verbatim.

## Routine prompt (paste verbatim)

```
Run the posthog-ux-discovery skill
(.claude/skills/posthog-ux-discovery/SKILL.md). If Skill({skill:
"posthog-ux-discovery"}) errors with "Unknown skill", read
.claude/skills/posthog-ux-discovery/SKILL.md with the Read tool and follow
its steps by hand instead of giving up. Query PostHog's MCP server for
friction signals (rage-clicks, funnel drop-off, unused features) across
web/'s key flows per the skill's steps, apply its genuine-finding
thresholds, and file or update one refined GitHub issue per genuine finding
via refine-issue. Never apply a UI change, never create a branch, never open
a PR — your only output is filed or updated issues (including the skill's
provisional funnel-drop-off bookkeeping issues). Do not ask me anything and
do not wait on a response — decide and act, or note in your final summary
why a step couldn't be completed, and move on. Call record_action with
mode=open, trigger_source=schedule, routine_name=posthog-ux-discovery before
pulling any data, and with mode=close (the id the open call returned) once
every flow has been checked, recording outcome no_action_needed, succeeded,
or failed and a short summary.
```

## Known platform constraints

- **#1624** — a fired session may not register repo-local skills; the
  prompt's read-the-file fallback covers it.
- **#1625** — a fired session may lack authenticated GitHub access; issue
  filing relies on the attached GitHub connector's `mcp__github__*` write
  tools.

## Related

- `docs/adr-0024-posthog-product-analytics.md` — the PostHog integration.
- `README.md`'s "Apps MCP server" — `record_action`/`get_automated_actions`.
