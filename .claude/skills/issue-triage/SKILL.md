---
name: issue-triage
description: Triage open GitHub issues on tools.xdoubleu.com — merge duplicates, add a short refined summary to each issue body, apply existing labels, set Priority/Status on the "Main Project" board, and split oversized issues into real linked GitHub sub-issues. Use whenever the user asks to "triage issues", "refine the issues", "clean up the backlog/tracker", "merge duplicate issues", "prioritize issues", or "go through the open issues".
---

# Issue Triage

Reads every open issue, uses judgment (not string-matching) to spot duplicates,
gives each surviving issue a one-line summary, the right labels, and a
Priority/Status on the project board, and breaks up any issue that's really
several pieces of work into linked sub-issues.

## Config

- repo: `xdoubleu/tools.xdoubleu.com` (default; accept an `owner/repo` argument to override)
- project board: "Main Project" — `gh project list --owner xdoubleu` if the number below ever stops matching
  - project number: `8`, owner: `xdoubleu`
  - has `Status` (Backlog / Ready / In progress / In review / Done) and `Priority` (P0 / P1 / P2) single-select fields — don't recreate these, look up their current field/option ids each run with `gh project field-list 8 --owner xdoubleu --format json`, since ids aren't worth hardcoding in a doc that can drift.
- marker label: `triaged` — create once if it doesn't exist yet:
  `gh label create triaged --repo <repo> --color ededed --description "Reviewed by issue-triage skill"`

## Priority rule

This is the user's own ordering, always apply it over any other instinct:

- **P0** — fixes or restores something that already works today but is currently broken.
- **P1** — improves existing, working functionality.
- **P2** — brand-new features that don't exist yet.

A shiny new feature never outranks a broken thing. When an issue is ambiguous, ask which bucket it means rather than guessing.

## Steps

1. **Pull the issues.**
   `gh issue list --repo <repo> --state open --json number,title,body,labels,url --limit 200`
   Include already-`triaged` issues in this read (you need them as context for duplicate matching) but don't touch them again.

2. **Read all of it yourself and reason about it.** Don't write a similarity script — spotting "these two are the same underlying ask" is exactly the kind of judgment call an LLM is better at than fuzzy string matching. For each untriaged issue decide:
   - Duplicate of another open issue? (same underlying problem/request — not just same area of the code)
   - A one-line summary of what it's actually asking for
   - Type label: `bug` / `enhancement` / `feature` / `chore` / `documentation` — whichever already-existing label fits; the repo has these, don't invent new ones
   - App label if scoped to one app: `books` / `games` / `recipes` / `mealplans` / `shoppinglist` / `todos`, otherwise `platform` or `infra`
   - Priority per the rule above
   - Whether it actually bundles 2+ separable pieces of work — if so, list candidate subtask titles

3. **Show the plan, then wait.** A short table: issue# → duplicate-of/keep, summary, labels, priority, proposed subtasks. Closing issues and rewriting bodies is hard to undo, so get a go-ahead before executing even though the general behavior (auto-comment-and-close dupes, rewrite descriptions) is pre-approved — the *plan* is what needs a look, not the mechanism.

4. **Execute, P0 first, then P1, then P2.**

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
   - Apply type label + app label + the `triaged` marker in one call:
     `gh issue edit <num> --repo <repo> --add-label "bug,books,triaged"`
   - Add to the project (if not already on it) and capture the item id:
     `gh project item-add 8 --owner xdoubleu --url <issue-url> --format json --jq .id`
   - Set Priority and Status using the field/option ids you looked up in step 0. Status mapping: P0/P1 → `Ready`, P2 → `Backlog` (new features wait; "Ready" implies someone could pick it up now, which isn't true for a feature that hasn't earned a slot yet).
     ```
     gh project item-edit --project-id <PVT_id> --id <item-id> --field-id <field-id> --single-select-option-id <opt-id>
     ```

5. **Split bundled issues into real sub-issues** (this repo's project board already has "Parent issue" / "Sub-issues progress" fields — use GitHub's native relationship, not a markdown checklist):
   ```
   gh issue create --repo <repo> --title "<subtask title>" --body "Split out of #<parent>."
   gh api repos/<repo>/issues/<parent>/sub_issues -X POST -f sub_issue_id=<child_numeric_id>
   ```
   Note `sub_issue_id` wants the numeric database id, not the issue number — get it with `gh api repos/<repo>/issues/<num> --jq .id`. Label and prioritize each new subtask the same way as step 4.

6. **Close with a short summary**: duplicates closed, issues refined, subtasks created, counts by priority. A chat message is enough — no need to write a report file unless asked.

## Notes

- Don't re-triage an issue that already has the `triaged` label — if the user wants one redone, they'll remove the label or say so explicitly.
- If the project board, its number, or its field names ever change, re-derive them from `gh project list --owner xdoubleu` / `gh project field-list <n> --owner xdoubleu` rather than trusting anything cached from a previous run.
