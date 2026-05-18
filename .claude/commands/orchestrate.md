# Orchestrate

You are an orchestrator. Execute the following workflow for the user's task.

**User's task:** $ARGUMENTS

---

## Phase 1 — Plan (Sonnet)

Spawn a **Plan** sub-agent with `model: "sonnet"`. The plan agent must NOT write any code or make any changes — planning only.

Instruct it to:

1. **Analyze** which files and functions the task requires changing.

2. **Check existing coverage** on those files. Identify uncovered branches and functions that will be touched.

3. **Check for duplicate / extractable code**: scan for the same pattern appearing in 2+ places across the codebase. If the task requires writing code that already exists elsewhere, add an extraction todo BEFORE the implementation todo. Check both backend (Go) and frontend (templates) for duplicates.

4. **Check file sizes**: for every file that will be touched or created, estimate the resulting line count. Any file over ~300 lines — including `.go` source files, `.templ` template files, and `*_test.go` test files — must include a split plan that divides by concern before new code is added. For test files, split by the feature or handler group under test (e.g. separate `tasks_crud_test.go`, `tasks_search_test.go`). For `.templ` files, split by UI concern (e.g. separate `views_list.templ`, `views_form.templ`). For Go files with large JS string constants that cannot be split, extract the constants to a companion plain `.go` file.

5. **Produce a complete, ordered todo list** separated into two explicit sections:

   ### BACKEND
   Go, SQL, migrations, services, repositories, handlers, jobs, config, and their tests.

   ### FRONTEND
   Next.js, React, TypeScript, Tailwind, shadcn/ui components, Jest tests, browser-facing assets, and their tests.

6. **Sequence each section** in this order:
   - **Coverage increase first**: tests for currently-uncovered code in files that will be touched (so regressions are caught the moment functional changes land).
   - **Deduplication / extraction**: extract any shared patterns before writing code that would duplicate them.
   - **Functional implementation**: the actual changes required by the task.
   - **Tests for new code**: targeting ≥80% coverage on all changed code.

7. **Act as QA reviewer**: for every file the plan touches, identify untested paths, error branches, edge cases, and boundary conditions. Add test todos for each gap found.

8. **Documentation impact**: determine whether the change affects project structure, new packages, CLI commands, Make targets, shared services, or architecture conventions. If yes, add explicit todos at the **end** of the BACKEND section to update `CLAUDE.md` and `README.md` with those changes.

Capture the full plan before proceeding.

---

## Phase 2 — Execute (Haiku, parallel)

Once you have the full plan, spawn **TWO general-purpose sub-agents with `model: "haiku"` IN PARALLEL in a single message**:

- **BACKEND agent**: receives the BACKEND section of the plan. Executes fully — implementation first, then tests. Trust the plan; do not self-review or re-plan.
- **FRONTEND agent**: receives the FRONTEND section of the plan. Executes fully — implementation first, then tests. Trust the plan; do not self-review or re-plan.

**FRONTEND agent must additionally:**

- Make all UI changes mobile-friendly and fully responsive. Verify layouts across small, medium, and large viewports. Use relative units and responsive breakpoints. Avoid fixed widths.
- Minimize user friction: prefer SWR/React state updates over full page reloads, reduce click count, use optimistic UI where appropriate, avoid unnecessary loading states. Use Next.js App Router patterns (Server Components for initial data, Client Components only where interactivity is needed).

If one section is empty, skip that agent. Wait for **both** to complete before proceeding.

---

## Phase 3 — Token Efficiency Review (Sonnet)

After both execution agents complete, spawn a **Sonnet** sub-agent. Pass it:

- The original user task
- The full plan from Phase 1
- A summary of what the BACKEND and FRONTEND agents did (files touched, tools called, approaches taken)
- The path to `.claude/commands/orchestrate.md` and `CLAUDE.md`

Instruct it to:

### A. Analyze token waste

Review the workflow just completed for inefficiencies:

- Were files read in full when only a slice was needed?
- Were searches repeated that a single targeted lookup could have resolved?
- Did agents explore speculatively instead of reading known paths from CLAUDE.md?
- Were generated files (e.g. `*_templ.go`) read when the source file would have sufficed?
- Were test or build runs repeated unnecessarily?
- Did the plan under-specify something, forcing agents to re-discover information?
- Was code written that duplicates an existing pattern elsewhere in the codebase?
- Were files left over the size threshold without being split?
- Were types or helpers defined locally that already existed in a shared package?

### B. Apply improvements (mandatory — do not just report)

1. **Apply high-value, low-effort code changes** surfaced by the analysis:
   - Create shared helpers or packages that eliminate confirmed duplication.
   - Split oversized files if the task just touched them and the split is mechanical.
   - Create new Make targets or scripts that replace recurring multi-step workflows.
   - Create new `.claude/commands/` slash commands for recurring agent workflows.

2. **Update `CLAUDE.md`** for any structural change resulting from this task (new package, new convention, new Make target, changed architecture).

3. **Update `.claude/commands/orchestrate.md`** to prevent repeated mistakes:
   - If an agent made a systematic error, tighten the relevant Phase instruction.
   - If the plan under-specified something that caused wasted re-discovery, strengthen Phase 1 to require that detail.
   - Keep both files generic — no project-specific lists or hardcoded file paths.

4. **Report** every file changed and the specific edit made, plus any suggestions that require user action.

---

## Report

After all three phases complete, report to the user:

1. What the plan prescribed
2. What each execution agent did
3. Token efficiency changes applied
4. Any items requiring user attention
