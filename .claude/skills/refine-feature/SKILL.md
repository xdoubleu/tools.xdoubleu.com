---
name: refine-feature
description: Turn a rough feature idea into a refined parent (epic) board task plus vertically-sliced sub-tasks in Backlog, without writing any code. Use when the user describes a feature that is clearly bigger than one PR — a new app, a new integration, a multi-page flow — or asks to "refine this", "break this down", "create tasks for this", or "make an epic". For a single self-contained task use `refine-task` directly instead.
---

# Refine Feature

Turn one rough feature description into a parent task plus sub-tasks, each
shippable via `start-task`/`finish-task`. **No code** — the deliverable is the
task tree. The board has no hierarchy, so the tree is labels and links.

Use `refine-task` for every task created; it owns the repo id, labels and the
P0/P1/P2 rule. Never redefine those here.

## 1. Ground it first

- **Search for duplicates** (`factory_list_tasks` with `query`, every column).
  An existing open task is a parent candidate, not a reason to stop.
- **Read the code it lands in:** which `api/apps/*` app (or a new one), which
  `web/app/*` route, what it extends. Name real files and packages.
- **Verify every external dependency** (auth, licensing, coverage, rate
  limits) via a subagent that returns only the distilled answer. Mark
  anything unverified as such in the task body and give it its own spike
  sub-task.

## 2. Grill the user

Run the `grilling` skill, round by round — unconditional. Ask only choices
that change the task set (naming of a new app/schema, domain scope,
build-vs-adopt, first-cut slices). Decide the rest yourself and record it in
the parent's `## Decisions`.

Feature-track PRs pass an automated gate and then wait for the owner's
review; a human also tests the output at the end
([`convention-feature-review-policy`](../../../docs/convention-feature-review-policy.md)).
Be exhaustive; an unresolved choice ships as whatever the implementer assumed.

## 3. Write the parent

Sections: `## Context`, `## Decisions`, `## Scope` / `## Not this`, and a
sub-task map by title (each child's one-line purpose and ordering constraints).
The parent carries rationale; sub-tasks carry work. A body can't be edited, so
the map has titles; ids go in a closing comment (step 5).

## 4. Slice sub-tasks vertically

- **One sub-task = one PR** that merges alone. If it can't, fold it into its
  dependency.
- **Slice by user-visible capability, not layer.** Foundational work with no
  user-facing half (a client package, a schema) is allowed only when the
  parent names the later slice that consumes it.
- **Size** against `finish-task`'s manual-review threshold (~150–200 changed
  lines / 8 files).
- **The first slice is usable on its own** in the browser, not scaffolding.
- Order them; each body names its dependency by title and id.
- A sub-task ending in a user-only step (third-party registration, API key,
  webhook, deploy secret) states it under an exact `## Follow-up: your turn`
  heading — `finish-task` keys on it to move the task to "Needs you".

## 5. Wire up the tree

- Create the parent through `refine-task` with the label `epic`, then each
  child in order with `Part of <parent title> (<parent id>)` and its
  dependency's title and id at the top of the body, and the label
  `epic:<slug>` (the parent's too, so `factory_list_tasks(label)` lists the
  tree). Finish with a comment on the parent (`factory_comment_task`) listing
  every child's title and id.
- **Add the `feature` label to the parent and every sub-task**, alongside
  `refine-task`'s labels. Without it `finish-task` falls through to the
  maintenance track.
- Everything lands in Backlog; you can't make a task Ready, so say that the
  owner moves them.

## 6. Close the loop

- A feature adding an app, package, Make/npm target, or shared service
  triggers `AGENTS.md`'s "Docs Impact" rule — give it a slice or fold it into
  one.
- Report the tree as a short ordered list (ids + titles) plus anything
  unresolved.
- **Stop.** Implementation starts separately with `start-task`.
