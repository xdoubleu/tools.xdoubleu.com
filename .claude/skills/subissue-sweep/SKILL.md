---
name: subissue-sweep
description: Pull every open sub-issue of a given parent GitHub issue and dispatch one isolated subagent per sub-issue to fix it test-first (start-task, then a failing test that reproduces the bug, then the minimal fix, then finish-task), closing the parent once all sub-issues have landed. Use whenever the user asks to "fix all sub-issues of #N", "land #N", "clear out #N's sub-issues", or "close out the epic/spike #N".
---

# Sub-issue Sweep

Fix every open sub-issue of a parent issue (real GitHub sub-issues, not a body
checklist) with one isolated subagent each, then close the parent. This
session only triages and dispatches.

## Steps

1. **Pull sub-issues:** `gh issue view <N> --json subIssues`, filtered to
   `state == "OPEN"`. None → say so and stop; this skill never splits work
   (that's `refine-feature`/`issue-triage`).

2. **Overlap:** note file/area overlap in each affected prompt, but keep one
   subagent and one PR per sub-issue. Pass down any file/function-level
   detail you already have.

   **Shared mutable value check:** if several sub-issues must *edit* the same
   single value (version/feature-flag constant, registry map, shared struct
   literal), every merge forces a rebase + full CI on the rest — use the
   serialized merge in step 3a.

3. **Dispatch one `Agent` per open sub-issue, in parallel (one message),
   `isolation: "worktree"`.** Each prompt is self-contained and tells the
   subagent to:
   - Read `AGENTS.md` and the subtree's `AGENTS.md` first.
   - Read the sub-issue's **live** state, body, and all comments. `REOPENED`
     means not done, even if a PR once merged. If prior attempts exist,
     follow `start-task` step 1 and record "Why attempt #N failed".
   - `start-task`, adopting the existing sub-issue.
   - Follow the `tdd` skill: **write a failing test reproducing the bug
     first**, confirm red, write the minimal fix, confirm green. If the test
     won't fail, and the issue isn't `REOPENED` or re-asked in a post-merge
     comment, report "already fixed" and close the sub-issue with an
     explanatory comment — no PR, no scope creep.
   - `finish-task` (lint, ≥80% changed-line coverage, build for `web/`,
     non-draft PR with `Fixes #<n>`, auto-merge decision, CI green).
   - Not invent scope; on ambiguity make the smallest reasonable call and
     note it in the PR. **Empty body** → report it needs scoping, no PR.
   - Report: sub-issue number, root cause, already-fixed or not, PR URL,
     CI/merge status.

   With a shared-mutable-value overlap, tell affected subagents they may open
   the PR early for CI, but must stop at "ready to merge" and not rebase past
   a sibling's merge or merge until told.

3a. **Serialized merge** (only when flagged): in ready order, rebase one PR
    onto the default branch, resolve the conflict, confirm CI green, merge,
    then the next. Implementation stays parallel.

4. **Don't poll** — completion notifications arrive per agent.

5. **Close the parent** once every sub-issue has a merged PR or already-fixed
   close: `gh issue close <N> --comment "<one line per sub-issue: number →
   outcome/link>"`. Any blocker (needs input, CI not passing) → leave the
   parent open and report it.

6. **Summarize to the user:** one line per sub-issue (number → approach →
   PR/status) and the parent's final state.

## Related

`ready-issues-sweep` covers the whole Ready column; `monitoring-sweep` covers
`/monitoring` problems.
