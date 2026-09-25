---
name: refine-feature
description: Turn a rough feature idea into a refined parent (epic) GitHub issue plus vertically-sliced sub-issues on the board, without writing any code. Use when the user describes a feature that is clearly bigger than one PR — a new app, a new integration, a multi-page flow — or asks to "refine this", "break this down", "create issues for this", or "make an epic". For a single self-contained issue use `refine-issue` directly instead.
---

# Refine Feature

Turn one rough feature description into a parent issue plus sub-issues, each
shippable via `start-task`/`finish-task`. **No code** — the deliverable is the
issue tree.

Use `refine-issue` for every issue created; it owns repo/board config, labels,
and the P0/P1/P2 rule via `.claude/github-triage.config.json`. Never redefine
those here.

## 1. Ground it first

- **Search for duplicates** (`search_issues` + a `list_issues` scan). An
  existing open issue is a parent candidate, not a reason to stop.
- **Read the code it lands in:** which `api/apps/*` app (or a new one), which
  `web/app/*` route, what it extends. Name real files and packages.
- **Verify every external dependency** (auth, licensing, coverage, rate
  limits) via a subagent that returns only the distilled answer. Mark
  anything unverified as such in the issue body and give it its own spike
  sub-issue.

## 2. Grill the user

Run the `grilling` skill, round by round — unconditional. Ask only choices
that change the issue set (naming of a new app/schema, domain scope,
build-vs-adopt, first-cut slices). Decide the rest yourself and record it in
the parent's `## Decisions`.

For feature-track work this is the **only** pre-merge human checkpoint —
every PR under a feature epic auto-merges unreviewed
([`convention-feature-review-policy`](../../../docs/convention-feature-review-policy.md)).
Be exhaustive; an unresolved choice ships as whatever the implementer assumed.

## 3. Write the parent

Sections: `## Context`, `## Decisions`, `## Scope` / `## Not this`, and a
sub-issue map (each child's one-line purpose and ordering constraints). The
parent carries rationale; sub-issues carry work.

## 4. Slice sub-issues vertically

- **One sub-issue = one PR** that merges alone. If it can't, fold it into its
  dependency.
- **Slice by user-visible capability, not layer.** Foundational work with no
  user-facing half (a client package, a schema) is allowed only when the
  parent names the later slice that consumes it.
- **Size** against `finish-task`'s manual-review threshold (~150–200 changed
  lines / 8 files).
- **The first slice is usable on its own** in the browser, not scaffolding.
- Order them; each body names its dependency.
- A sub-issue ending in a user-only step (third-party registration, API key,
  webhook, deploy secret) states it under an exact `## Follow-up: your turn`
  heading — `finish-task` keys on it to move the issue to "Needs you".

## 5. Wire up the tree

- Create the parent, then each child with `parent_issue_number` (or attach
  via `sub_issue_write`). Run every issue through `refine-issue` for
  labels/Priority/Status.
- **Add the `feature` label to the parent and every sub-issue**, alongside
  `refine-issue`'s labels. Without it `finish-task` falls through to the
  maintenance track.
- The board is a personal project, so `list_issue_fields`/`field_filters`
  don't resolve its fields; read columns via `get_project_issues_by_status`.
  If nothing can write board fields, tell the user the manual step instead of
  reporting the issues as triaged.
- No Slack step — `finish-task` posts the epic summary.

## 6. Close the loop

- A feature adding an app, package, Make/npm target, or shared service
  triggers `AGENTS.md`'s "Docs Impact" rule — give it a slice or fold it into
  one.
- Report the tree as a short ordered list (numbers + titles) plus anything
  unresolved.
- **Stop.** Implementation starts separately with `start-task`.
