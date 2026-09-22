---
name: subissue-sweep
description: Pull every open sub-issue of a given parent GitHub issue and dispatch one isolated subagent per sub-issue to fix it test-first (start-task, then a failing test that reproduces the bug, then the minimal fix, then finish-task), closing the parent once all sub-issues have landed. Use whenever the user asks to "fix all sub-issues of #N", "land #N", "clear out #N's sub-issues", or "close out the epic/spike #N".
---

# Sub-issue Sweep

Orchestrates fixing every open sub-issue of a parent GitHub issue (an epic
or spike that has spawned real GitHub sub-issues, not a checklist in the
body) by fanning work out to isolated subagents, one per sub-issue, rather
than working through them serially in the main session — then closes the
parent once every sub-issue has landed.

## Why subagents, not inline fixes

Each sub-issue is an independently scoped unit of work once the parent has
spawned it — that's what distinguishes a sub-issue from an unrefined
backlog item. Per root `CLAUDE.md`'s "Delegating to Subagents" section, keep
each sub-issue's exploration/fix/PR cycle out of the main session's
context. Dispatching in parallel also means N sub-issues each taking roughly
the same wall-clock time finish in about that same time total, not N times
it.

## Steps

1. **Pull the parent's sub-issues**: `gh issue view <N> --json subIssues`
   (or the equivalent GitHub MCP/project tool), filtered to `state == "OPEN"`.
   If the parent has no sub-issues at all, say so and stop — this skill fans
   out existing sub-issues, it doesn't invent or split work itself (that's
   `refine-feature`'s/`issue-triage`'s job).

2. **Skim titles/bodies for genuine file or area overlap** between
   sub-issues (e.g. two sub-issues touching the same pipeline file) and note
   it in each affected subagent's prompt so it's aware a sibling agent is
   touching nearby code — but default to one subagent per sub-issue; don't
   merge sub-issues into one PR just because they're thematically related.
   If your own exploration already surfaced concrete file/function-level
   detail relevant to a sub-issue (e.g. from planning), pass that down in
   its prompt too — a fresh subagent has none of this session's context and
   would otherwise have to re-derive it.

   **Look specifically for a shared mutable value** — a version/feature-flag
   constant, a registry map, a shared struct literal, or any other single
   piece of state multiple sub-issues will all need to edit (not just read)
   — as distinct from generic file overlap. This is the overlap shape that
   actually causes repeated rebase churn: every sub-issue's PR that edits
   the same mutable value conflicts with every sibling PR that merges
   before it, and each conflict costs a full rebase + re-lint + re-test +
   re-CI cycle, not a one-line fix. Concrete cost from the #611 sweep: 7
   sub-issues all shared `currentKEPUBConverterVersion` (a version-gate
   constant) in one file; the last sub-issue's PR to land had to rebase and
   re-bump that constant 4 separate times as siblings merged underneath it
   one at a time, each rebase re-running full CI. If this shape is present,
   apply the serialized-merge step in step 3 below instead of merging all
   PRs in an uncoordinated race.

3. **Dispatch one `Agent` call per open sub-issue, in parallel (single
   message, one Agent block per sub-issue), each with `isolation:
   "worktree"`** so they don't collide on the same git working tree. Each
   prompt must be fully self-contained and must tell it to:
   - Read root `CLAUDE.md` (and the relevant subtree's own `CLAUDE.md`, e.g.
     `web/AGENTS.md` or `api/AGENTS.md`) first.
   - Read the sub-issue itself (its URL or number) for the actual scope —
     a title is a label, not the full spec. **Read its live state and
     comments, not a cached first look**: check `state`/`stateReason` and all
     comments, and treat `REOPENED`-with-comment as *not done* — a merged PR
     that once closed it is not proof it stays fixed (e.g. #1867 was
     merged-then-reopened). If prior attempts exist, follow `start-task` step
     1 and record a "Why attempt #N failed" analysis before continuing.
   - Run `start-task` to adopt the existing sub-issue into a fresh worktree
     (it already exists, so `start-task` should adopt it rather than filing
     a new one).
   - Follow the `tdd` skill: **write a failing test that reproduces the bug
     first** (a hand-crafted unit test, a new fixture-based end-to-end test,
     or whatever seam fits the bug), confirm it's actually red, then write
     the minimal fix, and confirm it goes green. **If the test doesn't fail**
     — the bug turns out to already be fixed by prior work — report that
     instead of inventing new changes or scope-creeping into an adjacent
     improvement, and close the sub-issue with a comment explaining why,
     rather than opening a PR. Only conclude "already fixed" after ruling out
     a reopen: confirm the sub-issue's live `state`
     is not `REOPENED` and that no post-merge comment re-opened the ask — a
     reopened issue is "still broken", not "already fixed".
   - Run `finish-task` (lint, ≥80% changed-line coverage, build if `web/`
     changed, open a non-draft PR with `Fixes #<n>`, decide on auto-merge,
     watch CI to green).
   - Not invent scope beyond the sub-issue — if it's ambiguous about
     approach, make the smallest reasonable judgment call and note it in the
     PR description rather than stalling, since these subagents run
     unattended. **The one exception is a sub-issue with an empty body** (a
     title and nothing else): that is not ambiguity to resolve, it's a
     missing spec. Report back that it needs scoping by the user and open no
     PR.
   - Report back: sub-issue number, root cause, whether it was already
     fixed, PR URL, CI/merge status.

   **If step 2 flagged a shared-mutable-value overlap**, tell every affected
   subagent to stop after implementation + local lint/test are green and
   report "ready to merge" *without* opening the final rebase/PR-merge
   itself — the orchestrating session serializes that part (see step 3a).
   A subagent can still open its PR early (to get CI running), but must not
   rebase past a sibling's merge or merge its own PR until told to.

3a. **When a shared-mutable-value overlap was flagged, merge those
    sub-issues' PRs one at a time, not in parallel.** For each one, in the
    order they became ready: have its subagent (or do it yourself) rebase
    onto the current default branch, resolve the (at most one, since
    siblings aren't landing concurrently anymore) conflict, confirm CI
    green, merge, *then* move to the next one. This trades a small amount
    of serial wall-clock time in the merge phase for avoiding the O(n²)
    rebase cost of n PRs all racing to land through the same shared value —
    the implementation/TDD phase before this point stays fully parallel
    regardless, since that part doesn't touch the shared value yet.

4. **Do not poll the subagents.** They run in the background and this
   session gets a completion notification per agent — use the time for
   other work or hand control back to the user rather than sleeping or
   re-checking.

5. **Once every sub-issue is resolved** (each subagent has reported either a
   merged PR or an already-fixed no-op close), close the parent issue:
   `gh issue close <N> --comment "<summary>"`, where the summary lists each
   sub-issue number, its outcome (merged PR link, or already-fixed), in one
   line per sub-issue. If any sub-issue got blocked (needs user input, or a
   PR that didn't pass CI), don't close the parent — report the blocker
   instead and leave the parent open.

6. **Report a final summary to the user** once all subagents have reported
   back and the parent is closed (or left open with blockers called out):
   one line per sub-issue (number → approach → PR URL/status), plus the
   parent's final state.

## Notes

- This skill's job is triage and dispatch, not fixing anything itself in
  the main session — if you catch yourself reading application code to
  diagnose a specific sub-issue here, that work belongs in a dispatched
  subagent instead.
- Distinct from `ready-issues-sweep` (every issue in the board's "Ready"
  column, unrelated to each other) and `monitoring-sweep` (operational/
  production problems off the `/monitoring` page) — this skill is scoped to
  one parent issue's own sub-issues and additionally closes that parent when
  done.
