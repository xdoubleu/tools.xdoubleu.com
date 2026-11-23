---
name: start-task
description: Begin a task from the factory board — read it, move it to In progress, create a fresh worktree, and plan. Use when asked to "start T-123" or to pick up a board task. Reads .claude/repo.config.json.
---

# Start Task

Generic opening move for a board task; wraps `task-worktree`.

## Config

Read `.claude/repo.config.json` at the repo root first (schema: this plugin's README). If the file is missing, is not valid JSON, or lacks a key this skill needs, stop and say exactly `repo.config.json: missing key "<dotted.path>"` (or `repo.config.json: invalid JSON` / `not found`). Never guess a default for a required key. Call the board tools by the names under `board.*`; tasks live on the factory board, there is no GitHub-issue mode. Required keys here: `repoId`, `board.getTask`, `board.moveTask`, `board.commentTask`.

1. Fetch the task with `board.getTask`. Read the body, comments and linked jobs.
2. Fit check: if it clearly cannot be done in one job (several areas, or a migration plus new API plus UI), do not start. Slice it with `board.createTask` (Backlog, `repoId`) and comment the ids on the task.
3. If the task is a spike, or labelled with no matching `labels.types`, say so; spikes are settled interactively, not by an unattended job.
4. Move it to `in_progress` with `board.moveTask` (skip under `FACTORY_JOB`; the runner owns status).
5. Invoke `task-worktree` (default branch from `defaultBranch`, else `main`). Name the branch `<task-id-lowercase>-<short-slug>`. Under `FACTORY_JOB` the runner already checked out the branch: do not create another.
6. Match the task's files against `areas[].paths` to learn which gates will apply, and note them in the plan.
7. Plan, then implement. Hand off to `finish-task` when done.
