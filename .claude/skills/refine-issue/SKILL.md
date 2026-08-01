---
name: refine-issue
description: Create or refine a single GitHub issue on tools.xdoubleu.com — summary, type/app labels, Priority, and Status on the "Main Project" board — and keep a "## Plan" section in its body in sync. Use when starting work that has no tracking issue yet, when a plan-mode plan needs to be recorded on an issue, or when development begins and an issue's Status should move to "In progress".
---

# Refine Issue

Single-issue version of the refinement `issue-triage` does in bulk: give one issue a
summary, the right labels, a Priority, and a Status on the project board. Also owns
keeping a `## Plan` section on the issue in sync with plan mode, and moving Status to
`In progress` once development actually starts.

`issue-triage` reuses the config/labels/priority-rule below rather than redefining
them — keep the two in sync if any of this changes.

## Config

- repo: `xdoubleu/tools.xdoubleu.com` (default; accept an `owner/repo` argument to override)
- project board: "Main Project" — `gh project list --owner xdoubleu` if the number below ever stops matching
  - project number: `8`, owner: `xdoubleu`
  - has `Status` (Backlog / Ready / In progress / In review / Done) and `Priority` (P0 / P1 / P2) single-select fields — don't recreate these, look up their current field/option ids each run with `gh project field-list 8 --owner xdoubleu --format json`, since ids aren't worth hardcoding in a doc that can drift.

## Priority rule

This is the user's own ordering, always apply it over any other instinct:

- **P0** — fixes or restores something that already works today but is currently broken.
- **P1** — improves existing, working functionality.
- **P2** — brand-new features that don't exist yet.

A shiny new feature never outranks a broken thing. When an issue is ambiguous, ask which bucket it means rather than guessing.

## Labels

- Type label: `bug` / `enhancement` / `feature` / `chore` / `documentation` — whichever already-existing label fits; the repo has these, don't invent new ones.
- App label if scoped to one app: `reading` / `games` / `recipes` / `mealplans` / `shoppinglist` / `todos`, otherwise `platform` or `infra`.

## Steps — create or refine an issue

1. If no tracking issue exists yet for the work: `gh issue create --repo <repo> --title <title> --body <body>`.
2. Write/update the body with a one-line summary up top, original text preserved below a divider:
   ```
   gh issue edit <num> --repo <repo> --body "$(printf '**Summary:** %s\n\n---\n\n%s' "$SUMMARY" "$ORIGINAL_BODY")"
   ```
3. Apply the type label and app label: `gh issue edit <num> --repo <repo> --add-label "bug,reading"`.
4. Add to the project board if not already on it, capture the item id:
   `gh project item-add 8 --owner xdoubleu --url <issue-url> --format json --jq .id`
5. Set Priority and Status using the field/option ids looked up in Config. Status on
   creation follows the same mapping `issue-triage` uses: P0/P1 → `Ready`, P2 →
   `Backlog` (new features wait; "Ready" implies someone could pick it up now, which
   isn't true for a feature that hasn't earned a slot yet).
   ```
   gh project item-edit --project-id <PVT_id> --id <item-id> --field-id <field-id> --single-select-option-id <opt-id>
   ```

## Steps — record a plan

Once a plan-mode plan exists for the issue's work, insert or replace a `## Plan`
section in the issue body — placed after the Summary line, before the original-body
divider. If a `## Plan` section already exists from a prior session, replace it
rather than appending a duplicate:
```
gh issue edit <num> --repo <repo> --body "$(printf '**Summary:** %s\n\n## Plan\n\n%s\n\n---\n\n%s' "$SUMMARY" "$PLAN" "$ORIGINAL_BODY")"
```

## Steps — move to in progress

When development actually starts (first code edit or commit on the branch, not just
issue creation), set Status to `In progress` via the same `gh project item-edit`
pattern as above, using the `In progress` option id from Config.

## Notes

- If the project board, its number, or its field names ever change, re-derive them
  from `gh project list --owner xdoubleu` / `gh project field-list <n> --owner xdoubleu`
  rather than trusting anything cached from a previous run.
