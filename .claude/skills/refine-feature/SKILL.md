---
name: refine-feature
description: Turn a rough feature idea into a refined parent (epic) GitHub issue plus vertically-sliced sub-issues on the board, without writing any code. Use when the user describes a feature that is clearly bigger than one PR — a new app, a new integration, a multi-page flow — or asks to "refine this", "break this down", "create issues for this", or "make an epic". For a single self-contained issue use `refine-issue` directly instead.
---

# Refine Feature

Takes one rough feature description and leaves behind a parent issue plus its
sub-issues, each shippable on its own via `start-task`/`finish-task`. Produces
**no code** — the deliverable is the issue tree.

This layers a decomposition step on top of the generic `refine-issue` skill
(from the `github-issue-triage` plugin, `xdoubleu/xdoubleu-claude-plugins`
marketplace). `refine-issue` still owns repo/board config, the label lists,
and the P0/P1/P2 rule via `.claude/github-triage.config.json` — never redefine
any of that here. Use it for every individual issue this skill creates.

## 1. Ground it before writing anything

Refining from the feature description alone produces plausible-sounding
issues that don't survive contact with the code. Before drafting:

- **Search for duplicates** — `search_issues` plus a `list_issues` scan.
  An existing open issue is a parent candidate, not a reason to stop.
- **Read the code the feature lands in.** Which app under `api/apps/*`, or
  is it a new one? Which `web/app/*` route? What already exists that this
  extends? Name real files and packages in the issues.
- **Verify every external dependency.** If the feature is built on a
  third-party API, confirm what it actually returns — auth, licensing,
  realtime coverage, rate limits — rather than assuming. Delegate this to
  a subagent (root `CLAUDE.md`, "Delegating to Subagents"): it's noisy
  fetching whose distilled answer is all that belongs in the issues.
  Mark anything unverified as unverified **in the issue body**, and give it
  its own spike sub-issue rather than burying the risk in a design section.

## 2. Put the real decisions to the user

A refinement that silently picks for the user is the failure mode here. Ask
(via `AskUserQuestion`) only about choices that change the issue set —
naming of a new app/schema, how much of a domain is in scope, build-vs-adopt
for a dependency, whether a slice is in the first cut. Anything a careful
reader of the codebase would answer the same way, decide yourself and record
it in the parent's `## Decisions` section so it can be argued with later.

## 3. Write the parent issue

Match the house style of a refined issue here (see #1380 for the shape):
`## Context`, `## Decisions`, `## Scope` / `## Not this`, and a sub-issue map
listing each child with its one-line purpose and its ordering constraints.
The parent carries the design rationale; sub-issues carry the work. State
what is deliberately excluded — "not this" is what stops scope creep at
implementation time.

## 4. Slice sub-issues vertically

- **Each sub-issue is one PR** a session can take end-to-end through
  `start-task`/`finish-task`, and merges on its own without waiting for its
  siblings. If it can't merge alone, it's not a slice — fold it into its
  dependency.
- **Slice by user-visible capability, not by layer.** "Proto + repository +
  service + page for X" is a slice; "add all the database tables" is not —
  a layer-only issue can't be verified and blocks everything behind it.
  Foundational work that genuinely has no user-facing half (a client
  package, a schema) is allowed, but only when a later slice consumes it
  and the parent says which one.
- **Size against `finish-task`'s manual-review threshold** (~150–200 changed
  lines / 8 files). Consistently exceeding it means the slices are too big.
- **The first slice must be usable on its own** — the thinnest version of the
  feature the user could actually open in the browser, not scaffolding.
- Order them, and say in each body which sub-issue it depends on.

## 5. Wire up the tree

Create the parent first, then each child with `parent_issue_number` set (or
attach afterwards with `sub_issue_write`). Run every issue through
`refine-issue` for labels/Priority/Status so nothing lands off the board.

Note the known limitation from root `CLAUDE.md` (issue #1357): this board is
a **personal** project, so `list_issue_fields`/`field_filters` don't resolve
its fields. Board columns are read via `get_project_issues_by_status`; if no
tool in this session can *write* the board fields, say so plainly and leave
the user the one manual step, rather than reporting the issues as fully
triaged.

## 6. Close the loop

- If the feature adds an app, a package, a Make/npm target, or a shared
  service, root `CLAUDE.md`'s "Docs Impact" rule applies — give it a slice
  (or fold it into the slice that introduces it), don't leave it implicit.
- Report the tree to the user as a short ordered list with numbers and
  titles, plus anything still unresolved.
- **Stop there.** Refinement ends at the issue tree; implementing a slice is
  a separate task that starts with `start-task`.
