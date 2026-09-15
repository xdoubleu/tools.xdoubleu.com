---
name: ready-issues-sweep
description: Pull every issue in the "Ready" column of the GitHub project board and dispatch one isolated subagent per issue to fix it end-to-end (start-task through finish-task, PR opened). Use whenever the user asks to "go over Ready issues", "work through the board", "clear the Ready column", or "fix all Ready issues" — also the skill a nightly scheduled routine (issue #1447, `docs/spec-routine-ready-issues-executor.md`) runs unattended.
---

# Ready Issues Sweep

Orchestrates fixing every issue currently sitting in the "Ready" column of
the project board (`.claude/github-triage.config.json`'s `project.number`,
currently 8) by fanning work out to isolated subagents, one per issue,
rather than working through them serially in the main session.

## Why subagents, not inline fixes

Each Ready issue is an independent, already-scoped unit of work — that's
what "Ready" means on this board (as opposed to "Backlog", which still
needs `refine-issue`). Per root `CLAUDE.md`'s "Delegating to Subagents"
section, keep each issue's exploration/fix/PR cycle out of the main
session's context. Dispatching in parallel also means ten issues each
taking N minutes of wall-clock finish in ~N minutes total instead of 10×N.

## Interactive vs. unattended

Unlike `monitoring-sweep`, this skill's per-issue behavior doesn't change
between the two — a Ready issue is already scoped, so there's no
detection-only mode to fall back to; every dispatched subagent drives all
the way to an open PR either way (auto-merge behavior is exactly what
`finish-task` already decides, unchanged). The distinction only affects
step 0/step 7's `trigger_source` and step 6's final report: **interactive**
(a human asked for the sweep, this session, right now) reports the summary
back to that human; **unattended** (issue #1447's nightly routine —
`docs/spec-routine-ready-issues-executor.md` — invokes this skill with no
human watching) has nothing to report to, so the summary instead feeds
step 7's close call. Whichever prompt invoked this skill states which mode
applies; if it doesn't say, default to interactive. Either way, nothing in
this skill's steps ever ends on a question with no one there to answer it.

## Steps

0. **Open the run record.** Call `record_action(mode: "open", trigger_source:
   "schedule" in unattended mode / "manual" in interactive mode,
   routine_name: "ready-issues-executor")` before pulling the board. This is
   what makes the run show up in `get_automated_actions` history at all —
   nothing else observes an unattended routine running. Keep the returned
   `id`; the final step needs it to close the row. In interactive mode this
   is still worth doing (cheap, keeps the history complete) but isn't the
   point of the exercise the way it is for the nightly routine.

1. **Pull the Ready column**:
   `get_project_issues_by_status(status="Ready", project_number=<from config>)`.
   If this comes back empty on a project that should have issues, don't
   assume the column is actually empty — check
   `get_oauth_connections` for the `github` connection first: this tool
   needs `read:project` scope, and a connection made before that scope
   existed will silently return nothing until the user reconnects GitHub
   (claude.ai Settings → Connectors). Reconnecting is a human-only action
   this skill can't take itself, and nothing later in the sweep can proceed
   without it either — so file a tracking issue documenting the missing
   scope and end the sweep for this run, rather than reporting a false
   "nothing to do" or blocking on a question no one is there to answer.

2. **Skim titles for genuine overlap** (two issues that would touch the same
   files/area) and note it in each affected subagent's prompt so they're
   aware a sibling agent is touching nearby code — but default to one
   subagent per issue; don't merge issues into one PR just because they're
   thematically related (e.g. two different monitoring-page visualization
   complaints are still two separate fixes/PRs).

3. **Dispatch one `Agent` call per issue, in parallel (single message, one
   Agent block per issue), each with `isolation: "worktree"`** so they don't
   collide on the same git working tree. Each prompt must be fully
   self-contained (the subagent has none of this session's context) and
   must tell it to:
   - Read root `CLAUDE.md` (and the relevant subtree's own `CLAUDE.md`, e.g.
     `web/CLAUDE.md` or `api/CLAUDE.md`) first.
   - Read the issue itself (`issue_read` / the issue URL) for the actual
     scope — the board title is a short label, not the full spec.
   - Follow the repo's real workflow exactly: `start-task` (fresh worktree
     off main, refines/confirms the tracking issue — this issue already
     exists, so `start-task` should adopt it rather than filing a new one)
     → implement the fix → tests to the ≥80% changed-lines bar → `make
     lint`/`npm run lint` (whichever subtree changed) → `finish-task` (opens
     a non-draft PR closing the issue with `Fixes #<n>`, watches CI to
     green).
   - Not invent scope beyond the issue — if the issue is ambiguous about
     approach, make the smallest reasonable judgment call and note it in the
     PR description rather than stalling, since these subagents run
     unattended. **The one exception is an issue with an empty body** (a
     title and nothing else): that is not ambiguity to resolve, it's a
     missing spec. Report back that the issue needs scoping by the user and
     open no PR — a title permits too many readings for a guess to be worth
     more than the review time it costs.
   - Report back: issue number, root cause/approach, PR URL, CI status.

4. **Do not poll the subagents.** They run in the background and this
   session gets a completion notification per agent — use the time for
   other work (e.g. step 5) or hand control back to the user rather than
   sleeping or re-checking.

5. **Once dispatched, this file itself is the reusable artifact** — no
   further extraction needed unless a recurring sub-pattern emerges (e.g.
   if "Ready" sweeps keep needing the same cross-issue-conflict check, fold
   that logic in here rather than re-discovering it next time).

6. **Report a final summary** once all subagents have reported back: one
   line per issue (number → approach → PR URL/status), and call out any
   that got blocked or need a human decision. Interactive mode: this is the
   reply to the user. Unattended mode: there's no one to reply to — this
   summary instead becomes the `error`/`pr_url` detail (if any) passed to
   step 7's close call.

7. **Close the run record.** Call `record_action(mode: "close", id: <the id
   from step 0>, outcome: ...)` — `"no_action_needed"` if the Ready column
   was empty, `"succeeded"` if it dispatched subagents and every one
   resolved to an open PR without the sweep itself erroring out, `"failed"`
   if the sweep itself couldn't complete (the `read:project` scope issue
   from step 1, an MCP tool call erroring out, etc. — not the same as an
   individual issue getting blocked, which is a normal per-issue outcome
   reported in step 6, not a sweep failure). Set `pr_url` when the run
   produced exactly one PR worth linking; set `error` with a short reason
   when outcome is `"failed"`. This is the last step in every run of this
   skill — a run that opened the row in step 0 and never reaches this step
   leaves a permanently-open `automated_actions` row, which is its own
   detectable problem (issue #1443).

## Notes

- This skill's job is triage and dispatch, not fixing anything itself in
  the main session — if you catch yourself reading application code to
  diagnose a specific issue here, that work belongs in a dispatched
  subagent instead.
- Distinct from `monitoring-sweep` (operational/production problems off the
  `/monitoring` page — Sentry, CI, perf, security, storage) and
  `issue-triage`/`refine-issue` (grooming the *Backlog* column into
  well-scoped issues, not fixing already-Ready ones).
- The nightly routine that runs this skill unattended is documented in
  `docs/spec-routine-ready-issues-executor.md` — its exact prompt text,
  schedule, and required connectors, for reproducing the routine setup by
  hand in the claude.ai routines UI (it cannot be created via the
  trigger-creation API without silently losing the `tools-apps` connector —
  #1438's finding).
