---
name: issue-triage
description: Triage open GitHub issues in bulk — merge duplicates, add a short refined summary to each issue body, apply existing labels, set priority/status on the project board, and split oversized issues into real linked GitHub sub-issues. Use whenever the user asks to "triage issues", "refine the issues", "clean up the backlog/tracker", "merge duplicate issues", "prioritize issues", or "go through the open issues".
---

# Issue Triage

Reads every open issue, uses judgment (not string-matching) to spot
duplicates, gives each surviving issue a one-line summary, the right
labels, and a priority/status on the project board, and breaks up any issue
that's really several pieces of work into linked sub-issues.

## Config

Repo/project-board config, the type/scope label lists, and the priority
rule are defined once in `refine-issue`'s config file
(`.claude/github-triage.config.json` at the target repo's root) — read them
from there, don't redefine them here; if that file doesn't exist yet, run
`refine-issue`'s bootstrap questions first. This skill additionally uses:

- marker label: `config.markerLabel` (defaults to `triaged`) — create it
  once if it doesn't exist yet:
  `gh label create <marker> --repo <repo> --color ededed --description "Reviewed by issue-triage skill"`

## Steps

1. **Pull the issues.**
   `gh issue list --repo <repo> --state open --json number,title,body,labels,url --limit 200`
   Include already-marked issues in this read (you need them as context for duplicate matching) but don't touch them again.

2. **Read all of it yourself and reason about it.** Don't write a similarity script — spotting "these two are the same underlying ask" is exactly the kind of judgment call an LLM is better at than fuzzy string matching. For each unmarked issue decide:
   - Duplicate of another open issue? (same underlying problem/request — not just same area of the code)
   - A one-line summary of what it's actually asking for
   - Type and scope label per the label lists in `refine-issue`'s config
   - Priority per the rule in `refine-issue`'s config
   - Whether it actually bundles 2+ separable pieces of work — if so, list candidate subtask titles. Only propose a split when the pieces would plausibly ship as separate PRs. If the pieces touch the same file(s)/component and would naturally get fixed together in one pass, they're multiple small fixes to the same code area, not separate work — keep them as one issue. When unsure, prefer keeping it as one issue.
   - Is the issue's description missing any part of the change's blast
     radius (other interfaces/consumers built on top of the same
     functionality, UI implications, etc.)? If so, note it in the rewritten
     summary/body rather than silently refining an incomplete issue —
     bulk triage shouldn't block on asking the user, just flag it.
   - Is the issue genuinely ambiguous about *what work it authorizes* —
     empty body, two readings that produce materially different PRs? Collect
     these; you'll grill the user on them together in step 5 rather than
     guessing or refining them into something they didn't ask for.

3. **Execute directly, highest priority first** — no need to show the plan and wait first; the general behavior (auto-comment-and-close dupes, rewrite descriptions, relabel, reprioritize) is pre-approved. Report what was done afterward (step 6) rather than proposing it beforehand.

   Duplicates:
   ```
   gh issue comment <num> --repo <repo> --body "Duplicate of #<canonical>."
   gh issue close <num> --repo <repo> --reason "not planned"
   ```

   Everyone else:
   - Rewrite the body with the summary up top, original text preserved below a divider:
     ```
     gh issue edit <num> --repo <repo> --body "$(printf '**Summary:** %s\n\n---\n\n%s' "$SUMMARY" "$ORIGINAL_BODY")"
     ```
   - Apply type label + scope label + the marker label in one call:
     `gh issue edit <num> --repo <repo> --add-label "bug,area-a,triaged"`
   - If a project board is configured: add it to the project (if not already there) and capture the item id, then set priority/status using the field/option ids looked up per `refine-issue`'s config section.

4. **Split bundled issues into real sub-issues** — only the ones flagged in step 2 as plausibly separate PRs, not every issue with multiple bullets. Prefer GitHub's native sub-issue relationship over a markdown checklist if the project board surfaces it:
   ```
   gh issue create --repo <repo> --title "<subtask title>" --body "Split out of #<parent>."
   gh api repos/<repo>/issues/<parent>/sub_issues -X POST -f sub_issue_id=<child_numeric_id>
   ```
   Note `sub_issue_id` wants the numeric database id, not the issue number — get it with `gh api repos/<repo>/issues/<num> --jq .id`. Label and prioritize each new subtask the same way as step 3.

5. **Grill the user on everything you couldn't settle alone** — the
   ambiguous issues collected in step 2, and any split whose slice boundary
   is a real judgement call. Use the `grilling` skill if available;
   otherwise ask the frontier as one numbered round, each question with your
   recommended answer, and wait. Non-ambiguous relabel/reprioritize/summary
   work from step 3 does **not** wait on this — only the flagged issues do.
   Write each settled answer into the relevant issue body before closing out.

6. **Close with a short summary**: duplicates closed, issues refined, subtasks created, counts by priority. A chat message is enough — no need to write a report file unless asked.

## When `gh` isn't available (e.g. Claude Code on the web)

Same detection and general approach as `refine-issue`'s own "When `gh`
isn't available" section — read that first, this only covers what's
different here:

- **Step 1 (list issues)** maps onto the MCP issue-read/list tool's
  equivalent of `gh issue list --state open` — same fields
  (`number,title,body,labels,url`).
- **Step 3 (comment + close duplicates, rewrite/relabel everyone else)**
  maps onto the same issue write tool `refine-issue`'s fallback describes,
  including its `method`/`action`-style close reason.
- **Step 4 (split into sub-issues)** is two separate operations: creating
  the child issue (same issue-write tool, no gap) and linking it as a
  *native* GitHub sub-issue, which used `gh api .../sub_issues` directly
  because it's a newer relationship type without full `gh` subcommand
  coverage either. If the mounted GitHub MCP server has no sub-issue-link
  tool, that's a real capability gap — fall back to this skill's own
  documented degradation (a markdown checklist / plain issue reference
  instead of the native relationship) and say so in the closing summary,
  don't silently drop the link.
- **Project-board fields** in step 3 have the same gap `refine-issue`
  documents — state explicitly when they couldn't be set.

## Notes

- Don't re-triage an issue that already has the marker label — if the user wants one redone, they'll remove the label or say so explicitly.
- If the project board, its number, or its field names ever change, re-derive them from `gh project list --owner <owner>` / `gh project field-list <n> --owner <owner>` rather than trusting anything cached from a previous run.
