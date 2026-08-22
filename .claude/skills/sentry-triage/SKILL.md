---
name: sentry-triage
description: Check unresolved Sentry issues, file a GitHub issue describing the root cause and fix for each one not already tracked, and flag which Sentry issues to resolve once fixed. Use whenever the user asks to "check Sentry", "triage Sentry issues", "go through Sentry", or "file issues for Sentry errors".
---

# Sentry Triage

Reads unresolved Sentry issues via the `mcp__tools-apps__get_sentry_issues` MCP
tool (see CLAUDE.md's "Apps MCP server" section), investigates each one's
actual root cause in this codebase, and uses `refine-issue` to file/update a
GitHub tracking issue with a proposed fix. Once a tracking issue's fix has
already shipped (issue closed, still showing unresolved in Sentry — e.g. it
errored again before the deploy landed, or Sentry just hasn't re-triggered),
this skill closes the loop by calling `mcp__tools-apps__resolve_sentry_issue`
— the one mutating tool this MCP server exposes.

## Steps

1. **Pull unresolved Sentry issues**: call `mcp__tools-apps__get_sentry_issues`.
   Each issue has `Id`, `Title`, `Culprit`, `Permalink`, `Count`, `LastSeen`,
   `Level`, `Project`.

2. **Pull GitHub issues** (open and closed) for dedup:
   `gh issue list --repo xdoubleu/tools.xdoubleu.com --state all --json number,title,body,state,url --limit 200`.
   For each Sentry issue, check whether its permalink already appears in a
   GitHub issue body:
   - Referenced in an **open** issue → already tracked, don't refile it.
   - Referenced in a **closed** issue → the fix already shipped but Sentry
     still shows it unresolved; call `mcp__tools-apps__resolve_sentry_issue`
     with that issue's `Id` and move on. Don't re-investigate or refile.
   - Not referenced anywhere → untracked, continue to step 3.

3. **For each untracked Sentry issue, investigate before writing anything**:
   read the culprit/stack trace context, find the actual code path in this repo
   (use `ast-grep`, not `grep`, per CLAUDE.md), and identify the real root cause —
   don't paraphrase the Sentry title as the issue body.

4. **File or update a GitHub issue via `refine-issue`** (config for this repo
   lives in `.claude/github-triage.config.json`)
   for each one, so labels/Priority/Status/project-board placement stay
   consistent with every other issue in this repo. Body must include:
   - the Sentry permalink (so it's traceable back and dedup works next run)
   - the root cause, in this codebase's terms (file:line)
   - a proposed fix, not just a description of the symptom
   Priority follows `refine-issue`'s P0/P1/P2 rule — a Sentry issue is almost
   always P0 (something that already works is now erroring in production).

5. **Report back**: a short table of Sentry issue → outcome (GitHub issue
   filed/updated, already tracked and skipped, or resolved in Sentry because
   its fix had already shipped).

## Notes

- `resolve_sentry_issue` only fires for issues whose fix already merged
  (closed GitHub issue referencing the permalink) — never call it just because
  a fresh GitHub issue was filed for it; the bug isn't fixed yet at that point,
  only tracked.
- If `get_sentry_issues` returns empty, say so and stop — no need to touch GitHub at all.
