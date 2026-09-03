---
name: ready-issues-sweep
description: Pull every issue in the "Ready" column of the GitHub project board and dispatch one isolated subagent per issue to fix it end-to-end (start-task through finish-task, PR opened). Use whenever the user asks to "go over Ready issues", "work through the board", "clear the Ready column", or "fix all Ready issues".
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

## Steps

1. **Pull the Ready column**:
   `get_project_issues_by_status(status="Ready", project_number=<from config>)`.
   If this comes back empty on a project that should have issues, don't
   assume the column is actually empty — check
   `get_oauth_connections` for the `github` connection first: this tool
   needs `read:project` scope, and a connection made before that scope
   existed will silently return nothing until the user reconnects GitHub
   (claude.ai Settings → Connectors). Ask the user to reconnect rather than
   reporting a false "nothing to do".

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

6. **Report a final summary to the user** once all subagents have reported
   back: one line per issue (number → approach → PR URL/status), and call
   out any that got blocked or need a human decision.

## Notes

- This skill's job is triage and dispatch, not fixing anything itself in
  the main session — if you catch yourself reading application code to
  diagnose a specific issue here, that work belongs in a dispatched
  subagent instead.
- Distinct from `monitoring-sweep` (operational/production problems off the
  `/monitoring` page — Sentry, CI, perf, security, storage) and
  `issue-triage`/`refine-issue` (grooming the *Backlog* column into
  well-scoped issues, not fixing already-Ready ones).
