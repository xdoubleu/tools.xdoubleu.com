---
name: red-pr-repair
description: Drive every currently-red PR that carries the `dependencies` label or a `claude/`-prefixed branch back to green — diagnose the CI failure, push a fixing or unsticking commit (including the documented Codecov-stall workaround), or leave an explanatory comment on a PR that can't be fixed — never abandoning it. Also handles a red `main` branch itself, fired immediately by CI via the routine-fire webhook rather than waiting for the daily sweep. Use whenever the user asks to "fix the red PRs", "unstall the dependency PRs", "check why the Renovate/claude PRs are failing", or "run the red-PR repair sweep" — also the skill a morning scheduled routine (issue #1448, `docs/spec-routine-red-pr-repair.md`) runs unattended every day, and the skill `main.yml`'s `notify-main-ci-red` job fires immediately on a red push-to-main build (issue #1722).
---

# Red PR Repair

Orchestrates driving every red PR that Renovate or the `ready-issues-sweep`
executor routine (issue #1447) opened back to green, by fanning work out to
isolated subagents rather than working through PRs serially in the main
session. This closes the other half of #1338's self-healing loop:
`monitoring-sweep`/`ready-issues-sweep` get a CVE bump or a Ready-column
issue as far as an open PR — this skill is what keeps that PR from silently
stalling red overnight with no one watching it.

Issue #1722 added a second, distinct entry point (see "Entry point 2: a red
`main` branch" below) for `main`'s own tip going red, fired immediately from
CI rather than discovered on the next scheduled sweep.

## Entry point 1: the PR sweep (default)

## Scope

Only two kinds of PR are in scope — don't widen this to "every failing PR
in the repo":

- PRs carrying the `dependencies` label (Renovate/Dependabot bumps — the
  same label `IssueNotifierJob` already alerted on per issue #915, before
  Grafana took over alerting).
- PRs on a `claude/`-prefixed branch (opened by an agent — the executor
  routine from #1447, an interactive session, or a scheduled routine).

Anything else red (a human's own feature branch, a PR with no label and no
`claude/` branch) is out of scope — leave it alone. It has a human already
watching it; this routine's job is the two PR shapes that don't.

## Always unattended

Unlike `monitoring-sweep`, this skill has no interactive mode — it always
runs as if no one is watching, whether invoked by a human or by the morning
routine. There is no useful distinction here the way there is for a sweep
that can either fix things now or file an issue for later review: a red PR
already has an owner (Renovate or the agent that opened it) and a defined
success state (green CI, mergeable per the existing auto-merge rule), so
there's nothing to hand back to a human mid-run. Every decision point below
resolves to "act, or leave a comment explaining why not," never to a
question that waits on a response.

## Steps

1. **Open the run record.** Call `record_action(mode: "open",
   trigger_source: "schedule" for the morning routine / "manual" for an
   interactive invocation, routine_name: "red-pr-repair")` before pulling
   any PR data. Keep the returned `id` — the final step needs it to close
   the row. Nothing else observes this routine running; skipping this step
   leaves no record it ran at all (the same failure mode issue #1443 exists
   to catch).

2. **Pull the candidate set, MCP-only — never `make test` locally** (per
   the routine's own constraint: it reads CI state, it doesn't reproduce
   it). Two tools:
   - `get_failing_pull_requests` — open PRs with failing checks. Filter to
     ones carrying the `dependencies` label or whose branch starts with
     `claude/`.
   - `get_workflow_runs` — use this to pull the actual failing run(s) and
     their logs/conclusion for each candidate PR once you know which PRs to
     look at; it's the detail tool, `get_failing_pull_requests` is the
     candidate list.

   If the candidate set is empty, there's nothing to dispatch — skip
   straight to closing the run record with `outcome: "no_action_needed"`.

3. **Dispatch one `Agent` call per red PR, in parallel (single message, one
   Agent block per PR), each with `isolation: "worktree"`** so they don't
   collide on the same git working tree. Each prompt must be fully
   self-contained (the subagent has none of this session's context) and
   must tell it to:
   - Read root `CLAUDE.md` (and the relevant subtree's own `CLAUDE.md`,
     e.g. `api/CLAUDE.md` or `web/CLAUDE.md`) first.
   - State the exact PR number, branch name, and the specific failing
     check(s)/conclusion you already pulled — but verify against the PR's
     current state before acting, since CI state can change between the
     orchestrator's read and the subagent's start (a flaky job may have
     already been manually re-run, a sibling commit may have landed).
   - **Check out the PR's existing branch directly** (`gh pr checkout
     <number>`, or an equivalent `git fetch`/`git checkout` off the PR's
     head ref) inside its isolated worktree — do **not** run `start-task`.
     The branch, the tracking issue, and the PR all already exist; creating
     a new branch or a new tracking issue here would duplicate work rather
     than fix it. This is the one way this flow genuinely differs from
     `start-task`/`finish-task`'s usual opening move, and it's why the
     `finish-task`/`start-task` skills' own "how a subagent pushes a fix to
     an existing PR branch" note (their own docs) is the closer read than
     re-deriving the mechanics here.
   - **Diagnose before acting** — read the actual failing job's logs
     (`gh run view <run-id> --log-failed`, or the equivalent
     `get_workflow_runs` detail) rather than guessing from the check name
     alone. Then triage into exactly one of these three outcomes, in this
     priority order:
     1. **The stuck-Codecov case, first and explicitly.** If `ci-pass` (or
        the Codecov check itself) is failing with "Timed out waiting for
        Codecov to report," this is not a code problem — it's Codecov's
        check-suite stuck, and root `CLAUDE.md`'s CI section is explicit
        that **re-running the job never helps**; only a new commit clears
        it (the issue that opened this work cites this as
        `docs/spec-ci-pipeline.md` — that file no longer exists as of the
        docs consolidation in #1464, and the same guidance now lives
        directly in root `CLAUDE.md`'s CI section; read that, not the
        stale path). Push a trivial, functionally-inert commit to the PR
        branch (e.g. a comment/whitespace touch-up, or an empty
        `--allow-empty` commit if the repo's hooks tolerate one — prefer a
        real touch-up over `--allow-empty` since some pre-commit/CI setups
        reject truly empty commits) and push it. Do not re-run the check.
        This is documented as the single most likely way an unattended PR
        stalls forever, so check for it before investing time diagnosing
        anything else.
     2. **A determinable fix** (a dependency conflict a version bump
        introduced, a lint failure, a test that needs updating for a
        version bump's behavior change, a merge conflict with `main`).
        Make the fix, run the relevant lint/test commands locally to
        confirm (this is the one place local verification is expected —
        the *routine* doesn't run `make test` to go looking for problems,
        but a subagent fixing one already-identified failure absolutely
        should confirm its own fix before pushing), commit, and push to
        the PR branch.
     3. **No determinable fix.** Leave the PR exactly as it is — still
        open, still red — and post a `gh pr comment` explaining what was
        investigated, what's actually failing, and why it couldn't be
        resolved automatically (e.g. a genuine breaking change in the
        dependency bump that needs a human call on whether to accept it).
        **Never close, never abandon.** A PR this routine gives up on stays
        exactly as visible as one it never looked at.
   - **Never merge.** This routine doesn't introduce any new merge
     authority — whatever `finish-task`'s existing auto-merge rule already
     lets a small, code-only PR do (arm auto-merge, which then merges once
     CI goes green on its own) is untouched, but nothing here should call
     `gh pr merge` directly or force a merge that rule wouldn't already
     allow.
   - Report back: PR number, diagnosis, and outcome — one of "fixed and
     pushed," "unstuck via a trivial commit (Codecov stall)," or "left open
     with an explanatory comment," plus the commit SHA or comment URL.

4. **Do not poll the subagents.** They run in the background and this
   session gets a completion notification per agent. There is no other work
   to hand back to in unattended mode and no user to answer in interactive
   mode either — the orchestrating session stays open and reacts to each
   completion notification as it arrives (still without spin-polling), since
   the final step needs every PR's outcome before it can close the run
   record accurately.

5. **Report a final summary** once every subagent has reported back: one
   line per PR (number → diagnosis → outcome → commit/comment link). In an
   interactive invocation this is the reply to the user. In the unattended
   routine there's no one to reply to — this summary becomes the `error`
   detail (if any) passed to the next step's close call, and the full text
   is worth leaving as a comment on the run's own tracking context (there
   isn't a single tracking issue for the whole run the way `monitoring-
   sweep`'s unattended mode has one per workstream — the PRs themselves,
   via the comments left in step 3, are where the trail lives).

6. **Close the run record.** Call `record_action(mode: "close", id: <the id
   from step 1>, outcome: ...)` — `"no_action_needed"` if the candidate set
   was empty, `"succeeded"` if every candidate PR resolved to fixed, unstuck,
   or a left-with-comment report without the *routine itself* erroring out
   (a PR the routine correctly couldn't fix is still a successful run of the
   routine — that's outcome (c) working as designed, not a routine failure),
   `"failed"` if the routine itself couldn't complete (an MCP tool call
   errored out, a required connector wasn't available). Set `error` with a
   short reason when outcome is `"failed"`. This is the last step in every
   run of this skill — a run that opened the row in step 1 and never reaches
   this step leaves a permanently-open `automated_actions` row, which is its
   own detectable problem (issue #1443).

## Entry point 2: a red `main` branch

Triggered by `.github/workflows/main.yml`'s `notify-main-ci-red` job, which
POSTs to the existing `/webhooks/grafana-alert` route
(`api/cmd/api/routines_webhook.go`) the moment any push-to-main build/test/
deploy job fails — the same inbound webhook Grafana's `routine-fire` contact
point targets, just triggered directly from CI instead of waiting on
Prometheus/Grafana's evaluation cycle. `main.yml`'s CI section explains why
this matters: **`main` deploys without re-testing**, so a red push-to-main
job is closer in urgency to a production bug than to a red PR sitting
overnight — the daily 07:00 sweep (Entry point 1) would otherwise be the
first thing to notice it.

The fired routine's context (from the webhook payload's `text`) names the
commit SHA, the failing job(s), and the Actions run URL — read that instead
of re-deriving it from scratch. This entry point is diagnosed exactly like
Entry point 1's step 3 (Codecov-stall check first, then a determinable fix,
then a no-fix comment as the last resort) with one structural difference:

- **There is no existing PR or branch to check out.** Create a fresh branch
  off `origin/main` (`start-task`-style, or `git fetch origin main && git
  checkout -B fix-main-ci-<short-description> origin/main` if `start-task`
  isn't available in this session) at the failing commit's tip, make the
  fix there, and open a **new PR against `main`**, gated by normal CI —
  never push straight to `main` itself. That hard constraint is unchanged
  by how urgently this fires: `CLAUDE.md` is explicit that nothing bypasses
  PR review and CI on `main`. The "immediate" part is starting the
  diagnosis and getting a fix up for CI sooner, not skipping CI itself.
- If the Codecov-stall pattern applies here, it doesn't — that failure mode
  is specific to `ci-pass`, which only runs on `pull_request`/
  `workflow_dispatch`, never on `push` (`main.yml`'s `ci-pass` job:
  `if: always() && github.event_name != 'push'`). A red push-to-main job is
  always a real build/test/deploy failure, not a stuck Codecov check-suite.
- If no determinable fix exists, leave a comment on a new tracking issue
  (there's no PR to comment on yet) describing what was investigated and
  why it couldn't be resolved automatically, same as Entry point 1's
  no-fix case.
- **Still call `record_action` yourself, same as Entry point 1's steps 1
  and 6** — `trigger_source: "webhook"` (distinct from `"schedule"`/
  `"manual"`, so `get_automated_actions` can tell the two entry points
  apart), `routine_name: "red-pr-repair"`. This is a separate row from the
  one `routines.Client.Fire` (`api/internal/routines/client.go`) already
  opened with `trigger_source: "api"` before POSTing the webhook — that one
  only tracks whether api's own outbound HTTP call to fire this routine
  succeeded, not whether the routine's diagnosis itself did. Skipping this
  session's own open/close call would leave the actual work unrecorded, the
  same gap issue #1443 exists to catch.

## Notes

- Distinct from `monitoring-sweep` (the full `/monitoring` Issues page —
  Sentry, security alerts, storage, perf, and CI-red/dependency signals
  together) and `ready-issues-sweep`/its executor routine (issue #1447,
  turns a Ready-column issue into a *new* PR). This skill only ever acts on
  a PR that already exists — it never opens one.
- The morning routine that runs this skill unattended is documented in
  `docs/spec-routine-red-pr-repair.md` — its exact prompt text, schedule,
  and required connectors, for reproducing the routine setup by hand in the
  claude.ai routines UI (it cannot be created via the trigger-creation API
  without silently losing the `tools-apps` connector — issue #1438's
  finding, the same one `docs/spec-routine-nightly-maintenance-sweep.md`
  and this doc both work around).
