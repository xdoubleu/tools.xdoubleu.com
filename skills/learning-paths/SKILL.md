---
name: learning-paths
description: Author, refine, and track learning paths on tools.xdoubleu.com through the learningpaths `learningpaths_*` MCP tools. Use when the user asks to create a learning path or curriculum, refine an existing path's modules/items/resources, check off progress on a path item, or explain what a learning path is on tools.xdoubleu.com. Never use this to do repo/development work — it drives the product's own MCP tools, not the codebase.
---

# Learning Paths (tools.xdoubleu.com)

A learning path is a structured curriculum a user owns on
tools.xdoubleu.com: a `title`, a `goal` (what they want to be able to do
afterward), a freeform `routine` (how often they'll work on it, e.g.
"30 min every weekday morning"), and two ordered lists:

- **Modules** — the stages of the path, each with a `title` and an ordered
  list of **items**.
- **Items** — the concrete, checkable steps within a module. Each has a
  freeform `type` (e.g. `read`, `do`, `checkpoint`, `watch`, `practice`)
  and a `description` saying exactly what to do.
- **Resources** — freeform entries (a URL, "Book: …", a link to a books
  library entry or feeds item) that support the path but aren't steps to
  check off.

A user checks items off as they progress. Modifying a path means the
author agent itself calls the mutating MCP tools on the user's behalf.

## The MCP tools you drive

All tools are prefixed `learningpaths_` and operate only on the calling
user's own paths (scoped server-side by the OAuth'd user, never by a tool
argument). The tools-apps MCP server exposes them as:

| Tool | kind | Purpose |
| --- | --- | --- |
| `learningpaths_list_paths` | read | All of the user's paths. |
| `learningpaths_get_path` | read | One path's full tree: modules, items, resources. Returns the `id`s you need for updates/progress. |
| `learningpaths_get_progress` | read | Completion counts, overall and per module. |
| `learningpaths_create_path` | **write** | Create a new path with its full module/item/resource tree. |
| `learningpaths_update_path` | **write** | **Wholesale-replace** a path's `title`/`goal`/`routine`/`modules`/`resources`. It is not a partial patch — every field you pass replaces what's there, and `modules`/`resources` replace the prior lists entirely. |
| `learningpaths_record_progress` | **write** | Toggle a single item's `completed` flag by its `item_id` (from `learningpaths_get_path`), without resending the tree. |

### `create_path` / `update_path` argument shape

```
title       string          required
goal        string          optional
routine     string          optional  (freeform cadence, e.g. "30 min every weekday morning")
modules[]    ordered
  title     string          required
  items[]    ordered
    type        string      optional  (freeform verb: read / do / checkpoint / watch / practice …)
    description string      required
    completed   bool        optional  (default false)
resources[] freeform
  text      string          required  (a URL, "Book: …", etc.)
```

`items` and `resources` become the path's `Item`/`Resource` rows in the
order given — order matters and is preserved. `item_id`s are assigned
server-side; read them back with `learningpaths_get_path` after creating.

### `record_progress` argument shape

```
item_id    string   required  (from learningpaths_get_path)
completed  bool     required
```

## Authoring workflow

### 1. Elicit before you draft — never guess a curriculum uninvited

Ask for, at minimum: **the topic/goal**, the user's **current level** (from
scratch? some background?), **how much time they can spend**, and **on what
cadence** (daily lunch break? weekends?). A path built on a guessed goal or
a cadence the user can't keep is worthless, and it's persisted data — far
cheaper to confirm as conversation first than to delete later. If any of
these is genuinely not answerable, propose a sensible default and say so.

### 2. Draft the full tree and confirm it BEFORE creating

Before calling `learningpaths_create_path`, show the complete proposed
path — title, goal, routine, every module and item — and get the user to
sign off. A wrong curriculum costs far more once it's stored than when
it's still just text on a screen. This is an unconditional step for a
write tool.

### 3. Create

Call `learningpaths_create_path` with the confirmed tree.

### 4. Verify and report

Call `learningpaths_get_path` on the returned id and confirm the tree
landed as intended. Then report:

- the path **id**
- its web URL: `https://tools.xdoubleu.com/learningpaths/<id>`
- a one-line summary of what was created

### 5. Later turns: refine and track

- **Change something** → call `learningpaths_update_path`, resending the
  **entire** desired tree (it replaces wholesale — a partial patch will
  drop the fields you leave out).
- **Check an item off** → `learningpaths_get_path` for the current
  `item_id`, then `learningpaths_record_progress(item_id, completed)`.

## Curation rubric (what makes a path good)

Use these as defaults when drafting; deviate only with the user's explicit
agreement.

- **Goal framing** — the `goal` states what the learner can *do* afterward
  ("ship a small Rails API"), not just what they'll know.
- **Realistic routine** — phrase it as a real cadence with a time bound
  ("15 min every weekday morning"), not "take a course".
- **Prerequisite ordering** — modules progress from foundations to
  advanced; later modules build on earlier ones.
- **Actionable, checkable items** — not "learn X" but a concrete completing
  action: "read chapter 3 and summarize it", "build a to-do app endpoint",
  "watch part 2 and take notes". Each item is something the user can look
  back at and know they did it.
- **Shape** — roughly 3–6 items per module; one clear goal per path. If
  you're writing 20+ modules or 60+ items, suggest splitting into a more
  focused path.
- **Checkpoint items** — where a module has a natural mid-point or end,
  include a `checkpoint`-type item so progress has a way to register
  before the very end.
- **Resource curation** — prefer linking a real books-library entry or
  feeds item (via the resolved link fields) when one exists and matches,
  over a bare URL; otherwise a freeform "Book: …" or URL is fine. Only
  include resources that genuinely support the path.

## Guardrails

- **Never create empty modules** (a module with no items). Rework the tree
  or ask the user instead.
- **Always confirm before a write.** `create_path`/`update_path` mutate the
  user's real data; show the draft or the new full tree first.
- **Never pass the whole tree when a single-item toggle is all you need** —
  use `record_progress` for completion, `update_path` only for structural
  changes.
- **`update_path` replaces.** If you only mean to change the `routine`,
  still pass the full `title`/`goal`/`routine` **and** the full
  `modules`/`resources` arrays read from `get_path` — passing only the
  changed field drops everything you omitted.
- **Scoping is automatic.** These tools can only touch the calling user's
  own paths; don't try to pass another user's id or worry about it — the
  server rejects anything outside the caller.