---
name: refine-feature
description: Refine a feature on the factory board — grill the user on the whole feature, then slice it into tasks that each fit one job. Use when a request is bigger than one task. Reads .claude/repo.config.json.
---

# Refine Feature

Feature-level refinement; wraps `grilling`, then applies `refine-task` to each slice.

## Config

Read `.claude/repo.config.json` at the repo root first (schema: this plugin's README). If the file is missing, is not valid JSON, or lacks a key this skill needs, stop and say exactly `repo.config.json: missing key "<dotted.path>"` (or `repo.config.json: invalid JSON` / `not found`). Never guess a default for a required key. Call the board tools by the names under `board.*`; tasks live on the factory board, there is no GitHub-issue mode. Required keys here: `repoId`, `board.createTask`, `board.updateTask`, `board.commentTask`, `board.moveTask`, `labels`, `priorityRule`.

1. Invoke `grilling` on the feature: goal, users, non-goals, risks, order of delivery.
2. Slice it so each task fits one job (one area, one concern; never a migration plus RPCs plus UI together). Each slice is created with `board.createTask` in Backlog with label `epic:<slug>`, a type and scope from the config, and a priority by `priorityRule`.
3. Record dependencies between slices in each body and comment the slice ids on the parent.
4. Run `refine-task` on each slice before it moves to `ready`.
5. **Spikes are settled interactively during refinement and never move to Ready.** If an open question needs a spike, resolve it with the user now and record the answer; do not queue it for an unattended job.
