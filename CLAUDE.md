@AGENTS.md

# CLAUDE.md

Claude Code-specific guidance for this repository. The `@AGENTS.md` import on
line 1 loads the shared repository contract (architecture, commands,
conventions, MCP, production safety, git/worktree expectations, the docs
index) that applies to any coding agent working here. The import — not a
prose "read AGENTS.md first" pointer, and not Claude Code's native AGENTS.md
fallback — is what guarantees the contract is loaded: the fallback only
applies when no `CLAUDE.md` exists on the path, and a prose pointer depends
on Claude choosing to open the file. This file covers only what's genuinely
specific to Claude Code: skill orchestration, the enforcement hooks, and
Plan Mode.

Nothing below duplicates `AGENTS.md` — where a rule applies to any agent, it
lives there, not here.

## Delegating to Subagents

Prefer the `Agent` tool for noisy, multi-step, or bulk data-gathering — a
grep sweep across many files, a log/CI trawl, an MCP call whose raw output
is large (e.g. `mcp__tools-apps__get_logs`, `get_sentry_issues`), or
open-ended codebase exploration for a research question — rather than doing
it inline in the main session; have the subagent return only the distilled
findings. This is the same principle plan mode already applies via the
`Explore` agent type, extended to non-plan-mode work: keep raw,
mostly-discarded tool output out of the main context, not just the final
answer. (`AGENTS.md`'s equivalent guidance is the harness-neutral version of
this same rule — this section is the Claude-specific mechanism for it.)

## Starting a Task

Before exploring, reading code, or making any change, use the `start-task`
skill — it pulls latest `main`, creates a completely fresh worktree (never
edit in the main checkout or reuse an existing branch/worktree — a
`PreToolUse` hook denies an `Edit`/`Write`/`NotebookEdit` call whose target
path falls outside the active worktree), and creates/refines the GitHub
tracking issue via `refine-issue` before the first edit. The underlying
workflow it implements — fresh branch off up-to-date main, a tracking issue
before the first edit — is documented harness-neutrally in
[`docs/convention-task-lifecycle.md`](docs/convention-task-lifecycle.md);
this section and `start-task`/`refine-issue` are Claude Code's own
mechanism for it (worktree hook enforcement, the `github-issue-triage`
marketplace plugin, project-board Priority/Status fields).

**Exiting plan mode does not count as having started the task.** Whenever a
session does go through plan mode — by explicit request, or because the
user's client defaults to it — an approved plan is not a substitute for
`start-task`, which still runs before the first edit, with the
(already-grilled, see below) plan recorded in the tracking issue's `## Plan`
section. (`permissions.defaultMode: plan` used to be set repo-wide for
exactly this reason, until it started trapping unattended routine sessions
in Plan Mode with no one able to approve them out of it — see
`docs/adr-0014-start-finish-task-enforcement.md`.)

**Grill the scope during planning, before `ExitPlanMode` — not after.** When
a task goes through plan mode, invoke the `grilling` skill (the
round-by-round design-tree interview) against the plan during Phase 3
(Review), and settle its whole frontier before calling `ExitPlanMode`.
Record that grilling happened, and its settled decisions, in the Final Plan
file written in Phase 4 — a fresh session or subagent resuming the work has
no memory of the planning conversation and needs this to avoid re-grilling
from scratch. `start-task`'s `refine-issue` step then transcribes that
already-settled plan into the issue's `## Plan` section and states in one
line that it was pre-grilled during planning — it does not re-run the
round-by-round interview. `refine-issue`'s (and `issue-triage`'s/
`refine-feature`'s) own full grilling step still applies in full whenever a
task skips plan mode entirely, or when the plan changes materially after
approval (a reopened issue, a new `## Plan` revision, scope discovered
mid-implementation) — in those cases nothing was grilled yet, or what was
grilled is now stale. This is unconditional in both directions: a plan-mode
task cannot exit plan mode without a completed grilling round, and a
non-plan-mode (or materially changed) task cannot skip `refine-issue`'s own
grilling; there is no "looks trivial" escape hatch either way.

## Finishing a Task

Once a task's changes are complete, use the `finish-task` skill — it covers
lint, coverage, the web build, opening the PR (including the auto-merge
decision), watching CI to green, and the mandatory `session-retro` that
follows. Its lint/coverage/build/PR/CI-green requirements are the same ones
`docs/convention-task-lifecycle.md` states harness-neutrally; `finish-task`
is Claude Code's own mechanism for enforcing them (the `ship-pr`/
`session-retro` marketplace plugins, the auto-merge/feature-review-policy
decision, the Slack epic-completion summary).

**This is unconditional, and opening the PR is pre-authorized** — the user
does not need to ask for one, and a branch that is merely committed and
pushed is not a finished task. Should `finish-task` never load for any
reason, the floor is still: lint the areas that changed, ≥80% coverage on
changed code, `npm run build` for web changes, then a non-draft PR whose
body closes the tracking issue with a keyword (`Fixes #123`), then watch CI
to green.

An `ExitPlanMode` hook, a `Stop` hook, and a `PreToolUse` hook
(worktree-scope guard on `Edit`/`Write`/`NotebookEdit`) in
`.claude/settings.json` enforce this, and `make hooks/test` exercises them.
The `Stop` hook (`.claude/hooks/stop-check-unshipped-work.sh`) checks for an
existing PR via `gh` when available, and via a direct GitHub REST API call
otherwise — authenticated with whatever credential `git credential fill`
resolves — so it blocks a shipped-but-unopened PR whether or not `gh` is
present, including in a Claude Code on the web session →
[`docs/adr-0014-start-finish-task-enforcement.md`](docs/adr-0014-start-finish-task-enforcement.md).
OpenCode's counterparts live in `.opencode/plugins/repo-guard/` (a port of
the worktree guard, the session-start ff-merge, and — as an idle nudge
rather than a blocking stop — the unshipped-work check, sharing the Stop
hook's per-commit dedupe state) — see "OpenCode" below.

When a change adds or alters a page or component under `web/`, run the
`mobile-review` skill before `finish-task` — `docs/convention-ui-standards.md`'s
mobile-first rule is review-only, so nothing else in the pipeline looks at a
375px viewport.

**Two tracks decide whether a PR gets human review at all.** Maintenance
work (`enhancement`/`bug`/`chore`) keeps the tiered rule above unchanged.
Feature work (`feature` label) skips human review entirely —
`refine-feature` applies the label, `finish-task` reads it back and, when
present, runs an automated quality gate (`code-review` skill, Go `depguard`
+ web `dependency-cruiser` boundary lints, generated Mermaid diagrams,
diff-scoped mutation testing) in place of a reviewer, then auto-merges
unconditionally on green CI and posts a Slack summary once the whole
feature epic closes, via the `notify_slack` MCP tool. `ci-pass` is still
required on both tracks. See
[`docs/convention-feature-review-policy.md`](docs/convention-feature-review-policy.md).

`start-task`/`finish-task` are thin, project-specific wrappers around
generic skills (`task-worktree`, `ship-pr`, `session-retro`, `refine-issue`,
`issue-triage`) published from the `xdoubleu/skills`
marketplace repo — declared in `.claude/settings.json`'s
`extraKnownMarketplaces`/`enabledPlugins`. `refine-issue`/`issue-triage`'s
repo/project-board/label config lives in
`.claude/github-triage.config.json`, not in the skill files — edit that
file, not the plugin, when this repo's board/labels change.

## Other Automation Skills

`.claude/skills/` also carries a set of operational skills:
`dependabot-triage`, `monitoring-sweep`, `posthog-ux-discovery`,
`postmortem`, `ready-issues-sweep`, `red-pr-repair`, `refine-feature`,
`sentry-triage`, `subissue-sweep`. See each skill's own frontmatter
description for what it does. OpenCode loads them too (compatibility
discovery) and has equivalents for the mechanics they use — subagent
fan-out, the same `/apps/mcp` tools — so they can run interactively from
either harness. What stays Claude-only is the unattended trigger: the
claude.ai-scheduled routines (`docs/spec-routine-*.md`) that fire some of
them on a schedule.

## MCP

Connect Claude Code to the apps MCP server — OAuth is handled automatically,
no header needed:

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

See `AGENTS.md`'s MCP section for what the server exposes and the shared
OAuth 2.1 flow; this is only the Claude-specific connection command
(OpenCode's equivalent is the `mcp.servers` block in `opencode.json`).

## OpenCode

This repository also supports [OpenCode](https://opencode.ai) as a second,
fully independent development harness against OpenRouter models — see
`opencode.json`, `.opencode/plugins/`, and `AGENTS.md`. Nothing about that
setup changes how Claude Code works here: every hook, skill, and command in
this file and `.claude/` continues to apply exactly as before. The two
harnesses share `AGENTS.md`, the `/apps/mcp` server,
`docs/convention-task-lifecycle.md`, and the `.claude/skills/` project
skills (OpenCode reads them via compatibility discovery); they do not share
hooks or plugin mechanics, since those are each harness's own orchestration
mechanism. The generic marketplace skills `start-task`/`finish-task`
delegate to (`task-worktree`, `ship-pr`, `refine-issue`, `issue-triage`,
`session-retro`) plus mattpocock's `grilling`/`code-review` are installed
for OpenCode under `.agents/skills/` via `npx skills add` (tracked in
`skills-lock.json`, refreshed with `npx skills update`) — the same skills
Claude Code gets from its marketplace plugins, so `.claude/skills/` must not
grow its own copies.

OpenCode's enforcement counterpart is `.opencode/plugins/repo-guard/`: a
port of the worktree-scope edit guard, the session-start main ff-merge, and
the unshipped-work check (the last as an idle nudge rather than a blocking
stop, sharing the Stop hook's per-commit dedupe state). Two things remain
Claude Code-only with no OpenCode equivalent: the claude.ai-scheduled
routine automation the skills in "Other Automation Skills"
depend on, and the `ExitPlanMode` reminder hook (OpenCode's plan/build
agent switch has no hookable exit event).
