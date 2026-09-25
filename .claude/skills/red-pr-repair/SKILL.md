---
name: red-pr-repair
description: Drive every currently-red PR that carries the `dependencies` label or a `claude/`-prefixed branch back to green — diagnose the CI failure, push a fixing or unsticking commit (including the documented Codecov-stall workaround), or leave an explanatory comment on a PR that can't be fixed — never abandoning it. Also handles a red `main` branch itself, fired immediately by CI via the routine-fire webhook rather than waiting for the daily sweep. Use whenever the user asks to "fix the red PRs", "unstall the dependency PRs", "check why the Renovate/claude PRs are failing", or "run the red-PR repair sweep" — also the skill the morning scheduled routine (`docs/spec-routine-red-pr-repair.md`) runs unattended every day, and the skill `main.yml`'s `notify-main-ci-red` job fires immediately on a red push-to-main build.
---

# Red PR Repair

Keep agent- and Renovate-opened PRs from stalling red, by fanning out one
isolated subagent per PR. Always runs as unattended, even when a human invokes
it: every decision resolves to "act, or leave a comment explaining why not",
never a question.

## Entry point 1: the PR sweep (default)

**Scope — only these; leave every other red PR alone:**

- PRs labelled `dependencies` (Renovate/Dependabot).
- PRs on a `claude/`-prefixed branch.

### Steps

1. **Open the run record:** `record_action(mode: "open", trigger_source:
   "schedule"` (routine) / `"manual"`, `routine_name: "red-pr-repair")`. Keep
   the `id`.

2. **Pull candidates, MCP-only — never `make test` to go looking for
   problems.** `get_failing_pull_requests` is the candidate list (filter to
   scope); `get_workflow_runs` gives each candidate's failing run and logs.
   Empty set → step 6 with `"no_action_needed"`.

3. **Dispatch one `Agent` per red PR, in parallel (one message), `isolation:
   "worktree"`.** Each prompt is self-contained and tells the subagent to:
   - Read `AGENTS.md` and the subtree's `AGENTS.md` first.
   - Start from the PR number, branch, and failing checks gathered, but
     re-verify current PR state before acting.
   - **Check out the existing PR branch** (`gh pr checkout <number>`). Do
     **not** run `start-task` — the branch, issue, and PR already exist.
   - **Diagnose from the failing job's logs** (`gh run view <run-id>
     --log-failed`), not the check name. Then pick exactly one, in order:
     1. **Codecov stall** — `ci-pass` fails with "Timed out waiting for
        Codecov to report". Re-running never helps; push a trivial inert
        commit (prefer a real comment/whitespace touch-up over
        `--allow-empty`). Check this first.
     2. **Determinable fix** (dependency conflict, lint, test update for a
        bump's behavior change, merge conflict with `main`). Fix, verify
        locally with the relevant lint/tests, commit, push. **A merge
        conflict hit while pushing needs the same verification:** re-run the
        specific tests covering the conflicting files before pushing the
        resolution — never hand-write placeholder values in generated data.
     3. **No determinable fix** — leave the PR open and red, and `gh pr
        comment` what was investigated, what fails, and why it needs a human.
        **Never close, never abandon.**
   - **Never merge** or call `gh pr merge`; only `finish-task`'s existing
     auto-merge rule applies.
   - Report: PR number, diagnosis, outcome ("fixed and pushed" / "unstuck
     via trivial commit" / "left open with comment"), commit SHA or comment
     URL.

4. **Wait** for every completion notification without spin-polling.

5. **Summarize:** one line per PR (number → diagnosis → outcome → link).
   Human invocation: reply. Routine: it becomes the close call's `error`
   detail if any; the PR comments carry the trail.

6. **Close the run record, last:** `record_action(mode: "close", id, outcome)`
   — `"no_action_needed"` (empty set), `"succeeded"` (every PR fixed,
   unstuck, or commented — an unfixable PR is still success), `"failed"`
   (the routine itself broke; set `error`). **Only close after every push
   actually landed clean** — a push can hit a conflict needing another
   verified push. A conflict push that fails or can't be verified makes that
   PR `"failed"`. An unclosed row is its own detectable problem.

## Entry point 2: a red `main` branch

Fired by `main.yml`'s `notify-main-ci-red` job via `/webhooks/grafana-alert`
(`api/cmd/api/routines_webhook.go`) when any push-to-main build/test/deploy
job fails. `main` deploys without re-testing, so treat it as near-production
urgency. The webhook `text` names the commit SHA, failing job(s), and run URL
— start there.

Diagnose like step 3, except:

- **No existing branch:** create one off `origin/main` (`start-task`, or
  `git fetch origin main && git checkout -B fix-main-ci-<desc> origin/main`),
  fix, and open a **new PR** gated by normal CI. **Never push to `main`.**
- The Codecov stall can't apply — `ci-pass` doesn't run on `push`; this is
  always a real failure.
- No determinable fix → file a tracking issue describing the investigation.
- Still open/close your own run record with `trigger_source: "webhook"`,
  `routine_name: "red-pr-repair"`. The `"api"` row `routines.Client.Fire`
  opened only tracks the outbound fire call.

## Related

`monitoring-sweep` covers the whole `/monitoring` page; `ready-issues-sweep`
turns Ready issues into new PRs. Routine setup:
`docs/spec-routine-red-pr-repair.md`.
