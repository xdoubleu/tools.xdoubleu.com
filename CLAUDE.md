@AGENTS.md

# CLAUDE.md

Claude Code-specific mechanics only; the shared contract is `AGENTS.md`, imported above.

## Task lifecycle

- **Start:** run `start-task` before exploring or editing. A `PreToolUse` hook denies edits outside the active worktree. An approved plan does not count as started.
- **Plan mode:** run the `grilling` skill on the plan before `ExitPlanMode`, and record the settled decisions in the plan file. `refine-task` then transcribes that plan instead of grilling it again. Without plan mode, or after the plan changes materially, `refine-task`'s own grilling applies. There are no exceptions either way.
- **Finish:** run `finish-task` once changes are complete. Opening the PR is pre-authorized. If the skill won't load, the minimum is: lint the changed areas, keep changed-code coverage at or above 80%, run `npm run build` for web changes, `make lint/docs`, open a non-draft PR that links the board task with auto-merge armed, and get CI green.
- Run `mobile-review` before `finish-task` when a `web/` page or component changes.
- Every PR auto-merges on green CI (`finish-task` arms it); the owner reviews afterwards and checks tasks in Testing. `feature`-labelled work runs an automated gate before the PR opens → [convention-feature-review-policy](docs/convention-feature-review-policy.md).

Hooks in `.claude/settings.json` (tested with `make hooks/test`) → [adr-0014](docs/adr-0014-start-finish-task-enforcement.md):

- worktree edit guard
- `Stop` unshipped-work check, which works with or without `gh`

The `doc-budgets@xdoubleu-skills` plugin adds the `PostToolUse` doc-budget check, configured by `.claude/doc-budgets.json`; `make lint/docs` runs the same scripts at a pinned tag.

`start-task`, `finish-task`, `refine-task` and `refine-feature` come from the `task-lifecycle` plugin of `xdoubleu/skills`. Everything repo-specific (gates per area, board tools, labels, priority rule, PR wait command) lives in `.claude/repo.config.json`; they claim and file tasks through the `factory_*` tools of your agent account ([adr-0039](docs/adr-0039-agent-accounts.md) Setup). `.claude/github-triage.config.json` still feeds the GitHub-issue triage skills.

## Subagents

Use `Agent` for noisy bulk work (grep sweeps, log/CI trawls, large MCP outputs such as `get_logs`/`get_sentry_issues`), and have it return only distilled findings.

## Other skills

`.claude/skills/` also holds operational sweeps (see each skill's description). Factory routines (scheduled jobs on the board, run by the Mac runner) run them unattended → [adr-0052](docs/adr-0052-factory-scheduled-routines.md).

## MCP

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

## OpenCode

OpenCode shares `AGENTS.md`, the MCP server, and `.claude/skills/` (via `skills.paths` in `opencode.json`). Its copies of the marketplace skills live under `.agents/skills/` (`skills-lock.json`, `npx skills update`), so don't copy them into `.claude/skills/`. `.opencode/plugins/repo-guard/` ports the worktree guard, the session-start fast-forward of main, and the unshipped-work check (as a nudge). `.opencode/plugins/doc-budgets.js` is the plugin's doc-budget port; `make lint/docs` fetches its scripts, so point `DOC_BUDGETS_DIR` at them.
