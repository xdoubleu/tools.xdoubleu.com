@AGENTS.md

# CLAUDE.md

Claude Code-specific mechanics only; the shared contract is `AGENTS.md`, imported above.

## Task lifecycle

- **Start:** run `start-task` before exploring or editing. A `PreToolUse` hook denies edits outside the active worktree. An approved plan does not count as started.
- **Plan mode:** run the `grilling` skill on the plan before `ExitPlanMode`, and record the settled decisions in the plan file. `refine-issue` then transcribes that plan instead of grilling it again. Without plan mode, or after the plan changes materially, `refine-issue`'s own grilling applies. There are no exceptions either way.
- **Finish:** run `finish-task` once changes are complete. Opening the PR is pre-authorized. If the skill won't load, the minimum is: lint the changed areas, keep changed-code coverage at or above 80%, run `npm run build` for web changes, `make lint/docs`, open a non-draft PR with `Fixes #n`, and get CI green.
- Run `mobile-review` before `finish-task` when a `web/` page or component changes.
- `feature`-labelled work skips human review behind an automated gate and auto-merges → [convention-feature-review-policy](docs/convention-feature-review-policy.md).

Hooks in `.claude/settings.json` (tested with `make hooks/test`) → [adr-0014](docs/adr-0014-start-finish-task-enforcement.md):

- `ExitPlanMode` reminder
- worktree edit guard
- `Stop` unshipped-work check, which works with or without `gh`
- `PostToolUse` doc-budget check, which runs `scripts/lint_docs.sh` on an edited instruction file

`start-task`/`finish-task` wrap marketplace skills from `xdoubleu/skills` (`task-worktree`, `ship-pr`, `session-retro`, `refine-issue`, `issue-triage`). Board and label config for those skills lives in `.claude/github-triage.config.json`.

## Subagents

Use `Agent` for noisy bulk work (grep sweeps, log/CI trawls, large MCP outputs such as `get_logs`/`get_sentry_issues`), and have it return only distilled findings.

## Other skills

`.claude/skills/` also holds operational sweeps (see each skill's description). The claude.ai scheduled routines that fire them (`docs/spec-routine-*.md`) are Claude-only.

## MCP

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

## OpenCode

OpenCode shares `AGENTS.md`, the MCP server, and `.claude/skills/`. Its copies of the marketplace skills live under `.agents/skills/` (`skills-lock.json`, `npx skills update`), so don't copy them into `.claude/skills/`. `.opencode/plugins/repo-guard/` ports the worktree guard, the session-start fast-forward of main, the unshipped-work check (as a nudge), and the doc-budget check. It has no `ExitPlanMode` equivalent.
