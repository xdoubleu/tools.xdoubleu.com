---
name: finish-task
description: Finish a board task — run the configured gates for the changed areas, ship the PR, wait for CI, run the retro, and move the task. Use when a task's code is complete. Reads .claude/repo.config.json; under FACTORY_JOB runs offline and writes the Checks line.
---

# Finish Task

Generic closing move; wraps `ship-pr` and `session-retro`.

## Config

Read `.claude/repo.config.json` at the repo root first (schema: this plugin's README). If the file is missing, is not valid JSON, or lacks a key this skill needs, stop and say exactly `repo.config.json: missing key "<dotted.path>"` (or `repo.config.json: invalid JSON` / `not found`). Never guess a default for a required key. Call the board tools by the names under `board.*`; tasks live on the factory board, there is no GitHub-issue mode. Required keys here: `areas`, `board.moveTask`, `board.commentTask`, and `prWaitCommand` outside factory jobs.

## 1. Gates

List changed files (`git diff --name-only origin/<default>...HEAD` plus uncommitted). Match them to `areas[].paths`; every matching area's gates run: `lint`, `test`, `coverage`, `build`, `docs`. A gate listed in config always runs, even for a docs-only diff. Run `factory.finishCommand` instead when set and under `FACTORY_JOB`. Fix failures and rerun; never skip a gate or call it not-needed. A gate whose command cannot run in the environment is reported `not run`, with the reason.

## 2. Factory job (`FACTORY_JOB` set)

Offline: no `gh`, no push, no `prWaitCommand`. Commit locally. Write `$FACTORY_OUTCOME_PATH`: first line the PR title, then what was done and what is left. The final line is exactly one `Checks: <gate>=<pass|fail|not run>, ...` line (one entry per gate run, no text after it); keep exactly one when rewriting. Stop here.

## 3. Interactive

1. Invoke `ship-pr` (rebase, push, open PR, auto-merge decision).
2. Wait for CI with `prWaitCommand`; fix and repush on failure.
3. Invoke `session-retro` as the last step.
4. After merge, `board.moveTask` the task to `done` and `board.commentTask` the PR link.
