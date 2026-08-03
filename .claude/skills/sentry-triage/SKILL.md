---
name: sentry-triage
description: Check unresolved Sentry issues, file a GitHub issue describing the root cause and fix for each one not already tracked, and flag which Sentry issues to resolve once fixed. Use whenever the user asks to "check Sentry", "triage Sentry issues", "go through Sentry", or "file issues for Sentry errors".
---

# Sentry Triage

Reads unresolved Sentry issues via the `mcp__tools-apps__get_sentry_issues` MCP
tool (read-only — see CLAUDE.md's "Apps MCP server" section), investigates each
one's actual root cause in this codebase, and uses `refine-issue` to file/update
a GitHub tracking issue with a proposed fix. Resolving the issue *in Sentry* has
no automated path (the MCP server and `api/internal/sentryapi.Client` are both
read-only, no resolve endpoint wired up) — that step stays manual, done from the
Sentry permalink once the fix ships.

## Steps

1. **Pull unresolved Sentry issues**: call `mcp__tools-apps__get_sentry_issues`.
   Each issue has `Title`, `Culprit`, `Permalink`, `Count`, `LastSeen`, `Level`,
   `Project`.

2. **Pull open GitHub issues** for dedup: `gh issue list --repo xdoubleu/tools.xdoubleu.com --state open --json number,title,body,url --limit 200`.
   Skip any Sentry issue whose permalink already appears in an existing GitHub
   issue body — it's already tracked, don't refile it.

3. **For each untracked Sentry issue, investigate before writing anything**:
   read the culprit/stack trace context, find the actual code path in this repo
   (use `ast-grep`, not `grep`, per CLAUDE.md), and identify the real root cause —
   don't paraphrase the Sentry title as the issue body.

4. **File or update a GitHub issue via `refine-issue`** (`.claude/skills/refine-issue/SKILL.md`)
   for each one, so labels/Priority/Status/project-board placement stay
   consistent with every other issue in this repo. Body must include:
   - the Sentry permalink (so it's traceable back and dedup works next run)
   - the root cause, in this codebase's terms (file:line)
   - a proposed fix, not just a description of the symptom
   Priority follows `refine-issue`'s P0/P1/P2 rule — a Sentry issue is almost
   always P0 (something that already works is now erroring in production).

5. **Report back**: a short table of Sentry issue → GitHub issue filed/updated
   (or "already tracked, skipped"), plus a reminder that resolving the Sentry
   side is a manual click on each permalink once its fix has actually deployed —
   this skill has no write access to Sentry and doesn't mark anything resolved.

## Notes

- Don't invent a resolve step (API token, sentry-cli, etc.) — there is
  deliberately no Sentry write credential in this repo (`sentryapi.Client` only
  exposes `ListUnresolvedIssues`/`ListOrgs`/`ListProjects`). If that ever
  changes, this skill can grow a step 6 to call it; until then the manual click
  is the whole story.
- If `get_sentry_issues` returns empty, say so and stop — no need to touch GitHub at all.
