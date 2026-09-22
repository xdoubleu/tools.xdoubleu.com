---
name: monitoring-sweep
description: Sweep the /monitoring Issues page for every currently-open problem (Sentry errors, red CI/failing PRs, breaching perf alerts, security alerts, Grafana-managed alert state, orphaned storage) and dispatch one isolated subagent per problem to root-cause it, then close the loop on the monitoring page itself. Also runs a standing backstop that cross-checks recently-closed GitHub issues against still-unresolved Sentry issues, catching a skipped `finish-task` step 6. Use whenever the user asks to "check the monitoring page", "look into open issues", "fix what's flagged on /monitoring", or "do a monitoring sweep" — also the skill a nightly scheduled routine (issue #1446, `docs/spec-routine-nightly-maintenance-sweep.md`) runs unattended.
---

# Monitoring Sweep

Orchestrates working through everything the `/monitoring` Issues page
(`web/components/monitoring/IssuesClient.tsx`) currently reports as
non-zero, by fanning work out to isolated subagents rather than working
through things serially in the main session. This is the generalized form
of the workflow first run manually to clear out issues #1320-ish (Sentry
rate-limit noise, a red main branch, a 125s p95 perf regression, a
cache-poisoning CodeQL finding, orphaned R2 storage, and a batch of
Dependabot/secret-scanning alerts) — read that PR history if you want a
worked example of how granular the subagent split should be.

## Two invocation modes

- **Interactive** (a human is asking for the sweep, this session, right
  now — the common case): each dispatched subagent drives its problem all
  the way to a merged/mergeable PR (`start-task` → fix → `finish-task`,
  step 5 below).
- **Unattended** (issue #1446's nightly scheduled routine —
  `docs/spec-routine-nightly-maintenance-sweep.md` — invokes this skill
  with no human watching, and there is nothing to hand control back to in
  the meantime): this run is **detection-only**. Every dispatched subagent
  root-causes its problem and files (or updates) a refined GitHub tracking
  issue describing the root cause and a suggested fix, but never writes a
  code fix or opens a PR itself — turning a refined issue into a PR is a
  separate, later slice (issue #1447), the same way `ready-issues-sweep`
  turns a Ready-column issue into one. Driving straight from an
  automated, possibly-wrong reading of a signal to a merged fix with no
  human in the loop is exactly the failure mode detection-only mode
  exists to prevent. Whichever prompt invoked this skill states which mode
  applies; if it doesn't say, default to interactive.
- Unattended mode never ends on "ask the user" or waits on a human
  response — every decision point below that would otherwise need one
  instead resolves to "file/update a tracking issue and move on" (see step
  4's self-correcting instruction). A routine session that stalls waiting
  for input never gets one.

## Why subagents, not inline fixes

Each item on the Issues page is typically an independent root-cause
investigation touching a different app/subtree (web frontend noise vs. a Go
perf bug vs. a GitHub Actions workflow vs. dependency bumps). Per root
`CLAUDE.md`'s "Delegating to Subagents" section, keep that noisy,
mostly-unrelated investigation work out of the main session's context —
dispatch one subagent per item (or per tightly-related cluster of items) and
have each drive its own outcome independently.

## Steps

0. **Open the run record.** Call `record_action(mode: "open", trigger_source:
   "schedule" in unattended mode / "manual" in interactive mode,
   routine_name: "nightly-maintenance-sweep")` before pulling any data.
   This is what makes the run show up in `get_automated_actions` history at
   all — nothing else observes an unattended routine running. Keep the
   returned `id`; step 9 needs it to close the row. In interactive mode this
   is still worth doing (cheap, and keeps the history complete) but isn't
   the point of the exercise the way it is for the nightly routine.

1. **Pull every monitoring data source**, MCP-only. Three kinds of tool: the
   detail tools `IssuesClient.tsx`'s hooks call (the rows to act on),
   `get_grafana_alerts` for Grafana-managed alert-*rule* state, and
   `prom_query` for the raw Prometheus gauges behind those rules
   (`infra/grafana/provisioning/alerting/rules.yml`).
   - **`get_grafana_alerts` first** — call it before anything else. All
     alerting moved to Grafana in #1528 and a Grafana-managed alert never
     lands in Prometheus `ALERTS{}`, so `prom_query` alone cannot say
     whether e.g. `IssueGithubFailingPRs` or `IssueSecurityAlerts` is
     currently `Alerting` — only `get_grafana_alerts` can (issue #1564).
     This is also where CI-red and dependency-PR signals live now instead
     of in a removed app-side notifier job (`IssueNotifierJob`, retired in
     #1530) — a rule in `Alerting` state here is the ground truth for
     "something changed since the last sweep," and every detail tool below
     exists to explain *what*.
   - `get_sentry_issues` — unresolved errors. No Prometheus gauge since
     #1570; Grafana's `IssueSentryUnresolved` alert queries the Sentry API
     directly through the `grafana-sentry-datasource` plugin — cross-check
     against `get_grafana_alerts`' entry for that rule.
   - `get_failing_pull_requests` — open PRs with failing checks. Gauge:
     `github_failing_pull_requests`.
   - `get_workflow_runs` — filter to `event: push, branch: main` yourself and
     check whether the *latest* such run is a failure (a red main is worse
     than any single historical failure — CLAUDE.md's CI section explains
     why: main deploys straight to prod on every push, no re-test gate).
     Gauge: `github_workflow_run_failed{branch="main"}`.
   - `get_security_alerts` — Dependabot, code scanning, secret scanning.
     Gauge: `github_open_security_alerts` (labelled by severity).
   - `get_storage_stats` — `.latest.orphanCount`/`orphanKeys`. Gauges:
     `r2_orphaned_objects`, `r2_storage_bytes`.
   - `get_slow_transactions` — the `/monitoring` trending "currently slow"
     list, keyed off *live* Sentry p95 data (not an alert — the p95 alert is
     a Grafana rule on real histograms now, #1528 / ADR-0011). The name-shape
     classification and per-class thresholds it uses live in
     `web/lib/observability.ts` (exported `isSlowTransaction(transaction,
     p95DurationMs)`; thresholds in the `SLOW_TRANSACTION_THRESHOLDS_MS`
     const), mirroring `slow_transactions.go`.

   The six `github_*` / `sentry_*` / `r2_*` gauges are exported by
   `IssueSignalCollectorJob`
   (`api/internal/observability/jobs/issue_signal_collector.go`, #1539/#1540);
   `get_grafana_alerts` plus each gauge shows what Grafana is alerting on
   before you dig into the detail tools.

2. **Sentry resolution backstop.** Cross-check closed GitHub issues against
   still-unresolved Sentry issues — independent of anything the
   `/monitoring` page currently reports non-zero, since a Sentry issue whose
   fix shipped but whose `resolve_sentry_issue` call got forgotten doesn't
   move any page count on its own; closing it out is the only thing that
   would. This exists because `finish-task`'s own step 6 ("Resolve linked
   Sentry issues once merged") has no enforcement and has been missed three
   times now (#770, #775, #1786) — this step is the backstop that catches a
   miss after the fact, not a redesign of step 6 itself.
   - `list_issues(state: "CLOSED", since: <~14 days back>, fields: ["number",
     "title", "body", "closed_at", "html_url"])` against this repo. Two
     sweeps a day apart comfortably fall inside a 14-day window even if a run
     or two gets missed; going further back just re-checks issues an earlier
     sweep already confirmed clean.
   - For each closed issue whose body contains a Sentry permalink
     (`https://xdoubleu.sentry.io/issues/<id>/`), extract the numeric id and
     check it against step 1's `get_sentry_issues` unresolved list (already
     pulled this run — don't call it again).
   - Still unresolved despite the tracking issue being closed: don't resolve
     it off the closure alone — a closed issue can mean "won't fix" or
     "duplicate" as easily as "fixed," and resolving a Sentry issue whose
     underlying error is still live just hides a real problem. Only call
     `resolve_sentry_issue(issue_id)` when there's actual evidence the fix
     shipped and the error stopped recurring — the Sentry issue's own
     `lastSeen`/`count` predating the GitHub issue's `closed_at`, or the
     tracking issue's body/comments confirming the fix merged and the error
     stayed quiet since. When that evidence isn't there, leave it unresolved
     and flag it in step 8's final summary for human review instead.
   - This is a direct fix, not a root-cause investigation — call
     `resolve_sentry_issue` inline here rather than dispatching a subagent
     for it; there's no code left to write, since the code-side fix already
     merged. Only the ones you can't resolve need to surface anywhere else
     (the final summary), never a workstream of their own in step 3.

3. **Cluster into independent workstreams.** Don't dispatch one subagent per
   raw data row — group by root cause and by which part of the codebase
   they'll touch, so parallel subagents never collide on the same files:
   - One Sentry issue = usually one workstream (unless two issues share an
     obvious root cause).
   - A red main branch and any failing PRs whose failure looks the same
     shape = one workstream (fix main first, then re-check whether the PRs
     recover on rebase).
   - Each breaching perf alert / pathologically slow transaction = one
     workstream.
   - Small, unrelated infra fixes (a code-scanning workflow finding,
     orphaned-storage cleanup) can share one subagent *if* they touch
     unrelated files — but still have it open separate tracking
     issues/PRs (interactive mode) or separate tracking issues (unattended
     mode) per fix, never one PR/issue mixing unrelated changes.
   - Security alerts are their own thing — hand the whole batch to the
     `dependabot-triage` skill (see below) rather than inventing per-alert
     subagents yourself.

4. **Before dispatching, decide what's actually fixable now vs. what needs a
   human call.** Never dispatch a fix subagent for these — in either mode,
   file (or, for the security-alert cases, let `dependabot-triage` file) a
   tracking issue explaining why it needs a human decision, and keep
   dispatching everything else rather than blocking the whole sweep on it:
   - Secret-scanning alerts — a real leaked credential needs rotation by a
     human with access to the actual provider dashboard; a subagent can only
     investigate and report which alerts look like false positives vs. real.
   - Dependency CVEs with no upstream fix yet — nothing to build, just a
     dismiss-or-track decision.
   - Anything where "fix" would mean a breaking major-version bump, deleting
     production data, or another hard-to-reverse action.

   **Self-correcting instruction, for any signal (Grafana-sourced or
   otherwise):** when a signal can't be acted on for lack of context (the
   annotation/label doesn't say enough to root-cause it), or you judge it a
   false positive, don't silently skip it. File or update a GitHub issue
   proposing a concrete change to
   `infra/grafana/provisioning/alerting/rules.yml` — a different threshold,
   an added label, a richer annotation — that would have made this run
   actionable, and say what signal prompted it. That issue flows through
   #1447 like any other refined issue on the board. This is the same
   principle as `docs/convention-mcp-gap-first.md` (fix the tool before the
   incident) applied to alert-rule quality instead of MCP tool coverage: a
   sweep that can't act on a signal and says nothing about why produces the
   same recurring blind spot a missing MCP tool does.

5. **Dispatch one `Agent` call per workstream, in parallel, each with
   `isolation: "worktree"`** so they don't collide on the same git working
   tree. Each prompt must be fully self-contained (the subagent has none of
   this session's context), must state which mode applies, and must tell it
   to:
   - Read root `CLAUDE.md` (and the relevant subtree's own `CLAUDE.md`, e.g.
     `web/AGENTS.md` or `api/AGENTS.md`) first.
   - State the exact symptom/data you already gathered (IDs, counts,
     timestamps, URLs, the relevant `get_grafana_alerts` rule state) so it
     doesn't have to re-derive what you already know — but tell it to
     verify before acting, not just trust the summary.
   - Actually root-cause the problem in this codebase's terms before acting
     — no band-aids (e.g. don't silence a Sentry report without
     understanding why the error happens; don't add a timeout without
     knowing why something is slow).
   - **Interactive mode**: follow the repo's real workflow — `start-task`
     (tracking issue + fresh branch off main) → fix → tests to the ≥80%
     changed-lines bar → `make lint`/`npm run lint` → `finish-task` (PR
     closing the tracking issue, watch CI green).
   - **Unattended mode**: root-cause the problem, then use `refine-issue` (or
     `gh issue create`/`issue_write` if that skill isn't available to the
     subagent) to file or update one refined GitHub tracking issue per
     workstream — root cause, evidence gathered, and a concrete suggested
     fix, detailed enough that #1447's later execution pass doesn't have to
     re-investigate from scratch. Do not create a branch, do not open a PR.
     **Always include the `bug` label** (from
     `.claude/github-triage.config.json`'s type taxonomy) alongside whatever
     topic/scope labels apply — every workstream this skill investigates is,
     by definition, something already working that's now broken, matching
     the board's own P0/`bug` framing (`priorityRule`). Skipping this label
     means the issue sits in Ready forever: `ready-issues-executor` (#1744)
     filters to `bug`-labeled issues only and silently skips anything else,
     with no human ever seeing the skip (#1767 is the tracking issue for the
     first time this bit a batch of performance-regression findings).
   - Close the loop on the monitoring page's own *signal* data, in both
     modes, via whichever mutating tool applies — `resolve_sentry_issue` for
     Sentry (interactive mode: only after the fix is confirmed correct, not
     just pushed; unattended mode: only when the issue is a confirmed
     duplicate/false positive, never because a fix was filed, since no fix
     landed this run), `dismiss_security_alert` for
     Dependabot/code-scanning/secret-scanning alerts (see
     `dependabot-triage` for the per-alert-type dismissal judgement) — and
     report back which alerts it dismissed and why.
   - Report back a short summary: root cause, and either the fix + PR
     URL(s) (interactive) or the tracking issue number it filed/updated
     (unattended).

6. **Wait-for-completion behavior differs by mode.** Interactive: do not
   poll the subagents — they run in the background and this session gets a
   completion notification per agent, so continue other work (like step 7)
   or answer the user in the meantime rather than sleeping or re-checking.
   Unattended: there is no other work to hand back to and no user to answer
   in the meantime, so the orchestrating session stays open and reacts to
   each subagent's completion notification as it arrives (still without
   spin-polling) — step 9 needs every workstream's outcome before it can
   close the run record accurately.

7. **Once dispatched, consider whether a new reusable sub-skill is
   warranted.** If a workstream turned out to be a well-defined, likely-to-
   recur pattern (bulk security-alert triage was the first one — see
   `dependabot-triage`), extract it into its own `.claude/skills/<name>/`
   skill following the shape of `sentry-triage`/`postmortem`. Don't extract
   a sub-skill for a one-off investigation that's unlikely to recur in the
   same shape. Unattended-mode runs shouldn't create skills mid-run — note
   the idea in the run's own summary/report instead, for a human or a later
   interactive session to act on.

8. **Report a final summary** once all subagents have reported back: one
   line per workstream (issue → root cause → outcome → PR/tracking-issue
   link), step 2's Sentry-backstop resolutions and any it couldn't resolve
   and flagged for human review, any items that still need a human decision,
   any `rules.yml`-change issue(s) filed under the self-correcting
   instruction, and confirmation that Sentry issues were resolved / alerts
   dismissed as applicable. Interactive mode: this is the reply to the user.
   Unattended mode: there's no one to reply to — this summary instead
   becomes the `error`/`pr_url` detail (if any) passed to step 9's close
   call, and the full text is worth leaving in the last-filed/updated
   tracking issue's comments so it's visible without needing the routine's
   own transcript.

9. **Close the run record.** Call `record_action(mode: "close", id: <the id
   from step 0>, outcome: ...)` — `"no_action_needed"` if the sweep found
   nothing currently non-zero, `"succeeded"` if it dispatched workstreams
   and every one resolved to either a PR/mergeable state (interactive) or a
   filed/updated tracking issue (unattended) without the sweep itself
   erroring out, `"failed"` if the sweep itself couldn't complete (an MCP
   tool call errored out, a required connector wasn't available, etc. —
   not the same as an individual workstream needing a human call, which is
   a normal, successful outcome for that workstream). Set `pr_url` when
   interactive mode produced exactly one PR worth linking; set `error` with
   a short reason when outcome is `"failed"`. This is the last step in
   every run of this skill — a run that opened the row in step 0 and never
   reaches this step leaves a permanently-open `automated_actions` row,
   which is its own detectable problem (issue #1443).

## Notes

- This skill's job is triage and dispatch, not fixing anything itself in
  the main session — if you catch yourself reading application code to
  diagnose a specific issue's root cause here, that work belongs in a
  dispatched subagent instead.
- Distinct from `sentry-triage` (files GitHub issues for Sentry errors
  without necessarily fixing them) and `postmortem` (one deep dive on a
  single named incident, with a mandatory detection-gap assessment). This
  skill is the bulk, page-wide sweep that, in interactive mode, *does*
  drive fixes to green across whatever mix of issue types the page
  currently shows; in unattended mode it stops at a refined issue per
  problem, by design (issue #1446).
- The nightly routine that runs this skill unattended is documented in
  `docs/spec-routine-nightly-maintenance-sweep.md` — its exact prompt text
  and required connectors, for reproducing the routine setup by hand in the
  claude.ai routines UI (it cannot be created via the trigger-creation API
  without silently losing the `tools-apps` connector — #1438's finding).
