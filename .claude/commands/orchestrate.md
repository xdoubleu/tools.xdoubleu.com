You are an orchestrator. Execute the following two-phase workflow for the user's task.

**User's task:** $ARGUMENTS

---

## Phase 1 — Plan (Sonnet)

Spawn a **Plan** sub-agent with `model: "sonnet"`. The plan agent must NOT write any code or make any changes — planning only.

Instruct it to:

1. **Analyze** which files and functions the task requires changing.
2. **Check existing coverage** on those files. Identify uncovered branches and functions that will be touched.
3. **Produce a complete, ordered todo list** separated into two explicit sections:

   ### BACKEND
   Go, SQL, migrations, services, repositories, handlers, jobs, config, and their tests.

   ### FRONTEND
   HTMX, HTML templates (templ), CSS, JS, browser-facing assets, and their tests.

4. **Sequence each section** in this order:
   - **Coverage increase first**: tests for currently-uncovered code in files that will be touched (so regressions are caught the moment functional changes land).
   - **Functional implementation**: the actual changes required by the task.
   - **Tests for new code**: tests covering the newly added/changed functionality, targeting ≥80% coverage on all changed code.

5. **Act as a QA reviewer**: inspect existing test coverage for all files that will be touched, identify gaps, and append test todos to each section.

6. **Documentation impact**: For every file the plan touches, determine if the change affects project structure, new packages, CLI commands, Make targets, shared services, or architecture conventions. If yes, add explicit todos at the **end** of the BACKEND section to update `CLAUDE.md` (name the specific section) and `README.md`. These documentation todos are always last in the BACKEND section.

Capture the full plan (both sections) before proceeding.

---

## Phase 2 — Execute (Haiku, parallel)

Once you have the full plan, spawn **TWO general-purpose sub-agents with `model: "haiku"` IN PARALLEL in a single message**:

- **BACKEND agent**: receives the BACKEND section of the plan.
- **FRONTEND agent**: receives the FRONTEND section of the plan.

Each agent executes its section fully — implementation first, then its tests. Do not skip any step.

**FRONTEND agent additional requirements:**
- All UI changes must be mobile-friendly and fully responsive. Verify layouts work across small, medium, and large viewports. Use relative units and responsive breakpoints. Avoid fixed widths.
- Minimize user friction: prefer HTMX partial updates over full page reloads, reduce clicks required to complete actions, use optimistic UI updates where appropriate, avoid unnecessary loading states.

If one section is empty, skip that agent.

Wait for **both** agents to complete before proceeding.

---

## Phase 3 — Token Efficiency Review (Sonnet)

After both execution agents complete, spawn a **Sonnet** sub-agent. Pass it:

- The original user task
- The full plan from Phase 1
- A summary of what the BACKEND and FRONTEND agents did (files touched, tools called, approaches taken)

Instruct it to:

1. **Analyze token usage patterns** across the workflow just completed. Consider:
   - Were large files read in full when only a slice was needed?
   - Were searches repeated that could have been a single targeted lookup?
   - Did agents explore speculatively rather than reading known paths from CLAUDE.md or prior context?
   - Were templ-generated files (`*_templ.go`) read when the source `.templ` file would have sufficed?
   - Were test runs or builds repeated unnecessarily?
   - Did the plan lack enough specificity, forcing agents to re-discover things?
   - Did the plan omit a documentation-update step for a structural change? (new Make target, new package, new app, changed convention)

2. **Produce concrete suggestions** in two categories:
   - **Project changes**: additions or edits to `CLAUDE.md`, code structure, naming, or documentation that would let future agents find answers faster without searching.
   - **Tooling / command changes**: new Make targets, helper scripts, or `.claude/commands/` slash commands that would replace multi-step agent workflows with a single call.

3. **Rank suggestions by estimated token savings** (high / medium / low) and implementation effort (small / medium / large). Be specific — name the file, the section, and the exact text to add or change.

The agent reports suggestions only — it does not apply any changes.

---

## Report

After all three phases complete, report to the user:
1. A summary of what the plan prescribed
2. What each execution agent did
3. Any items that were skipped or require user attention
4. The token efficiency suggestions from Phase 3
