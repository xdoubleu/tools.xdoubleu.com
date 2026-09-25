---
name: sentry-triage
description: Check unresolved Sentry issues, file a GitHub issue describing the root cause and fix for each one not already tracked, and flag which Sentry issues to resolve once fixed. Use whenever the user asks to "check Sentry", "triage Sentry issues", "go through Sentry", or "file issues for Sentry errors".
---

# Sentry Triage

File a root-caused GitHub issue for every untracked unresolved Sentry issue,
and resolve Sentry issues whose fix already shipped.

## Steps

1. **Pull unresolved issues:** `mcp__tools-apps__get_sentry_issues` (`Id`,
   `Title`, `Culprit`, `Permalink`, `Count`, `LastSeen`, `Level`, `Project`).
   Empty → say so and stop.

2. **Dedup against GitHub** (open and closed):
   `gh issue list --repo xdoubleu/tools.xdoubleu.com --state all --json number,title,body,state,url --limit 200`.
   Match on the Sentry permalink in issue bodies:
   - Open issue → already tracked; skip.
   - Closed issue → fix shipped; `mcp__tools-apps__resolve_sentry_issue(Id)`
     and move on.
   - None → step 3.

3. **Investigate before writing:** trace the culprit/stack to the real code
   path (`ast-grep`) and find the root cause — don't paraphrase the title.

4. **File or update via `refine-issue`** (config
   `.claude/github-triage.config.json`). Body: Sentry permalink, root cause
   as `file:line`, proposed fix. Priority per `refine-issue` — almost always
   P0.

5. **Report** a table: Sentry issue → filed/updated, already tracked, or
   resolved.

Never resolve an issue just because a new GitHub issue was filed — only once
its fix merged.
