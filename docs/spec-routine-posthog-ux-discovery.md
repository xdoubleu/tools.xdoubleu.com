# Spec: weekly PostHog-insights-to-issues routine

- Status: code-side done (skill + this spec); the routine itself needs
  manual setup (see below), and produces no real output until issue #1638
  has shipped and had at least a full week to collect usage data.
- Issues: #1639, part of epic #1637. Depends on #1638.

## What this is

A claude.ai/code scheduled routine, running once a week against this
repository, whose prompt runs the `posthog-ux-discovery` skill
(`.claude/skills/posthog-ux-discovery/SKILL.md`): it queries PostHog's own
MCP server for product-analytics friction signals (rage-clicks, funnel
drop-off, unused features) across `web/`'s key flows and files a proposed
GitHub issue for each genuine finding. It **never** applies a UI change,
opens a branch, or creates a PR — a filed issue then goes through the
normal `refine-issue`/grilling pipeline like any other proposed feature,
consistent with epic #1627's human-reviewed-refinement checkpoint. This is
a separate, additional routine, parallel to (not a modification of) epic
#1338's maintenance routines (#1446/#1447/#1448).

This is a spec, not an ADR — there's no alternative design being weighed
here, just a reproducible record of exactly what to paste into the
claude.ai routines UI, per this repo's own convention that an ADR without
an "alternatives considered" section is a spec instead
(`docs/TEMPLATE-adr.md`).

## Why this can't be created through the trigger-creation API

Issue #1438 established empirically that the trigger-creation API silently
drops any connector the creating session doesn't itself hold. This routine
needs **three** connectors (`tools-apps`, GitHub, and PostHog's own MCP
server) — it must be created by hand at claude.ai/code/routines, in a
session that already holds all three, for the same reason every other
routine in this repo (`docs/spec-routine-nightly-maintenance-sweep.md`,
`docs/spec-routine-ready-issues-executor.md`,
`docs/spec-routine-red-pr-repair.md`) is recorded the same way.

## Manual setup steps

1. Go to `claude.ai/code/routines` → New routine.
2. Repository: `xdoubleu/tools.xdoubleu.com`.
3. Schedule: weekly, once a day on one fixed day (this consumes 1 of the 5
   daily routine runs available on that day — pick a day/time with headroom
   against the other scheduled routines in this repo).
4. Connectors — **all three required**, attached in the same session that
   creates the routine:
   - `tools-apps` — the MCP server at
     `https://tools.xdoubleu.com/api/apps/mcp` (`claude mcp add --transport
     http tools-apps https://tools.xdoubleu.com/api/apps/mcp`, or the
     equivalent claude.ai connector). Supplies `record_action` for the run
     record.
   - GitHub — to file/update tracking issues via `refine-issue`/`gh issue
     create`/`issue_write`.
   - PostHog's official MCP server — **confirm the exact connector URL and
     auth setup against PostHog's current documentation before wiring
     this**, since a hosted third-party MCP endpoint's exact address and
     required API key/scope are the kind of detail that drifts over time
     and shouldn't be hand-copied stale into this spec. Needs read access
     to the same PostHog Cloud (EU) project issue #1638 created.
5. Prompt: paste the block below **verbatim** into the routine's prompt
   field.

## Routine prompt (paste verbatim)

```
Run the posthog-ux-discovery skill
(.claude/skills/posthog-ux-discovery/SKILL.md). If Skill({skill:
"posthog-ux-discovery"}) errors with "Unknown skill", that means this
session's environment doesn't register repo-local skills (see issue
#1624) — fall back to reading .claude/skills/posthog-ux-discovery/SKILL.md
directly with the Read tool and follow its steps by hand instead of giving
up. Query PostHog's MCP server for friction signals (rage-clicks, funnel
drop-off, unused features) across web/'s key flows per the skill's steps,
apply its genuine-finding thresholds, and file or update one refined
GitHub tracking issue per genuine finding via refine-issue. Never apply a
UI change, never create a branch, never open a PR — your only output is a
filed or updated issue (including the skill's own provisional
funnel-drop-off bookkeeping issues). Do not ask me anything and do not
wait on a response — decide and act, or note in your final summary why a
step couldn't be completed, and move on. Call record_action with
mode=open, trigger_source=schedule, routine_name=posthog-ux-discovery
before pulling any data, and call it again with mode=close (using the id
the open call returned) once every flow has been checked, recording
outcome as no_action_needed, succeeded, or failed and a short summary of
what happened.
```

## Known platform constraints

Same two gaps #1438's spike found, tracked as #1624/#1625, apply here as
they do to every other routine in this repo:

- **#1624** — a routine-fired session may not register repo-local skills
  through the `Skill` tool. The prompt above has the same explicit
  read-the-file fallback every other routine's prompt uses.
- **#1625** — a routine-fired session may have no authenticated GitHub
  access by default, only an unauthenticated read-only clone. This routine
  only ever files/updates GitHub issues (never pushes code), so — per the
  partial re-confirmation recorded in
  `docs/spec-routine-nightly-maintenance-sweep.md` — the same
  `mcp__github__*` write-tool access other issue-filing routines rely on is
  expected to be available here too, via the GitHub connector attached in
  step 4 above.

A constraint specific to this routine: it produces **no genuine findings at
all** until issue #1638 has been live in production for at least a full
week — PostHog needs real traffic before any funnel/rage-click/feature-usage
query returns meaningful numbers. Runs before that point are expected to
report `no_action_needed` every time, not a bug in the routine or the skill.

## Related

- `.claude/skills/posthog-ux-discovery/SKILL.md` — the skill this routine
  runs, including the genuine-finding thresholds and the mutation boundary.
- `docs/adr-0024-posthog-product-analytics.md` — the PostHog Cloud (EU)
  integration this routine depends on for data.
- `README.md`'s "Apps MCP server" section — `record_action`/
  `get_automated_actions` (issue #1441), the run-history mechanism this
  routine uses the same way every other scheduled routine does.
