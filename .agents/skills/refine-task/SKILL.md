---
name: refine-task
description: Refine one factory-board task — grill its scope, then write the summary, type/scope labels, priority and plan into the task. Use when a task is vague, new, or before moving it to Ready. Reads .claude/repo.config.json.
---

# Refine Task

Single-task refinement; wraps `grilling`.

## Config

Read `.claude/repo.config.json` at the repo root first (schema: this plugin's README). If the file is missing, is not valid JSON, or lacks a key this skill needs, stop and say exactly `repo.config.json: missing key "<dotted.path>"` (or `repo.config.json: invalid JSON` / `not found`). Never guess a default for a required key. Call the board tools by the names under `board.*`; tasks live on the factory board, there is no GitHub-issue mode. Required keys here: `repoId`, `board.getTask`, `board.updateTask`, `board.moveTask`, `labels.types`, `labels.scopes`, `priorityRule`.

1. `board.getTask`; search `board.listTasks` (no filter on repo if a dependency may live elsewhere) for duplicates and dependencies.
2. Invoke `grilling` on the scope, round by round, until acceptance criteria are concrete. Ask; do not guess.
3. `board.updateTask`: a body with Summary, Plan and Done-when; one type and one scope from the config lists (`labels.fallbackScope` if none fits); priority by `priorityRule`.
4. If it does not fit one job, slice it (`board.createTask`, Backlog, label `epic:<slug>`) and comment the ids.
5. Move to `ready` only when criteria are concrete. **Spikes are settled interactively during refinement and never move to Ready**: keep them in Backlog or Needs you, and record the answer as a board comment or ADR.
