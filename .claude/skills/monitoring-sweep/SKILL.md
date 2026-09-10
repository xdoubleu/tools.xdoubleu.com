---
name: monitoring-sweep
description: Sweep the /monitoring Issues page for every currently-open problem (Sentry errors, red CI/failing PRs, breaching perf alerts, security alerts, orphaned storage) and dispatch one isolated subagent per problem to root-cause and fix it, then close the loop on the monitoring page itself. Use whenever the user asks to "check the monitoring page", "look into open issues", "fix what's flagged on /monitoring", or "do a monitoring sweep".
---

# Monitoring Sweep

Orchestrates fixing everything the `/monitoring` Issues page (`web/components/monitoring/IssuesClient.tsx`)
currently reports as non-zero, by fanning work out to isolated subagents
rather than fixing things serially in the main session. This is the
generalized form of the workflow first run manually to clear out issues
#1320-ish (Sentry rate-limit noise, a red main branch, a 125s p95 perf
regression, a cache-poisoning CodeQL finding, orphaned R2 storage, and a
batch of Dependabot/secret-scanning alerts) — read that PR history if you
want a worked example of how granular the subagent split should be.

## Why subagents, not inline fixes

Each item on the Issues page is typically an independent root-cause
investigation touching a different app/subtree (web frontend noise vs. a Go
perf bug vs. a GitHub Actions workflow vs. dependency bumps). Per root
`CLAUDE.md`'s "Delegating to Subagents" section, keep that noisy,
mostly-unrelated investigation work out of the main session's context —
dispatch one subagent per item (or per tightly-related cluster of items) and
have each drive its own fix to a merged/mergeable PR independently.

## Steps

1. **Pull every monitoring data source**, MCP-only. Two kinds of tool: the
   detail tools `IssuesClient.tsx`'s hooks call (the rows to act on), and
   `prom_query` for the Prometheus gauge + alert state behind the Grafana
   `service-health` alert group (`infra/grafana/provisioning/alerting/rules.yml`).
   Alerting moved wholesale to Grafana in #1528 — the old `get_alert_states`
   tool is gone; `prom_query('ALERTS{alertstate="firing"}')` is the read path
   for what is currently breaching.
   - `get_sentry_issues` — unresolved errors. No Prometheus gauge since #1570;
     Grafana's `IssueSentryUnresolved` alert queries the Sentry API directly
     through the `grafana-sentry-datasource` plugin.
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
   one `prom_query` for `ALERTS{alertstate="firing"}` plus each gauge shows
   what Grafana is alerting on before you dig into the detail tools.

2. **Cluster into independent workstreams.** Don't dispatch one subagent per
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
     issues/PRs per fix, never one PR mixing unrelated changes.
   - Security alerts are their own thing — hand the whole batch to the
     `dependabot-triage` skill (see below) rather than inventing per-alert
     subagents yourself.

3. **Before dispatching, decide what's actually fixable vs. what needs a
   human call.** Don't dispatch a fix subagent for these — file (or, for the
   security-alert cases, let `dependabot-triage` file) a tracking issue
   explaining why it needs a human decision, and keep dispatching everything
   else rather than blocking the whole sweep on it:
   - Secret-scanning alerts — a real leaked credential needs rotation by a
     human with access to the actual provider dashboard; a subagent can only
     investigate and report which alerts look like false positives vs. real.
   - Dependency CVEs with no upstream fix yet — nothing to build, just a
     dismiss-or-track decision.
   - Anything where "fix" would mean a breaking major-version bump, deleting
     production data, or another hard-to-reverse action.

4. **Dispatch one `Agent` call per workstream, in parallel, each with
   `isolation: "worktree"`** so they don't collide on the same git working
   tree. Each prompt must be fully self-contained (the subagent has none of
   this session's context) and must tell it to:
   - Read root `CLAUDE.md` (and the relevant subtree's own `CLAUDE.md`, e.g.
     `web/CLAUDE.md` or `api/CLAUDE.md`) first.
   - State the exact symptom/data you already gathered (IDs, counts,
     timestamps, URLs) so it doesn't have to re-derive what you already
     know — but tell it to verify before acting, not just trust the summary.
   - Actually root-cause the problem in this codebase's terms before
     patching — no band-aids (e.g. don't silence a Sentry report without
     understanding why the error happens; don't add a timeout without
     knowing why something is slow).
   - Follow the repo's real workflow: `start-task` (tracking issue + fresh
     branch off main) → fix → tests to the ≥80% changed-lines bar → `make
     lint`/`npm run lint` → `finish-task` (PR closing the tracking issue,
     watch CI green).
   - Close the loop on the monitoring page's own data via whichever mutating
     tool applies — `resolve_sentry_issue` for Sentry (only after the fix is
     confirmed correct, not just pushed), `dismiss_security_alert` for
     Dependabot/code-scanning/secret-scanning alerts (see
     `dependabot-triage` for the per-alert-type dismissal judgement) — and
     report back which alerts it dismissed and why.
   - Report back a short summary: root cause, fix, PR URL(s).

5. **Do not poll the subagents.** They run in the background and this
   session gets a completion notification per agent — continue other work
   (like building/updating skills, per step 6) or answer the user in the
   meantime rather than sleeping or re-checking.

6. **Once dispatched, consider whether a new reusable sub-skill is
   warranted.** If a workstream turned out to be a well-defined, likely-to-
   recur pattern (bulk security-alert triage was the first one — see
   `dependabot-triage`), extract it into its own `.claude/skills/<name>/`
   skill following the shape of `sentry-triage`/`postmortem`. Don't extract
   a sub-skill for a one-off investigation that's unlikely to recur in the
   same shape.

7. **Report a final summary to the user** once all subagents have reported
   back: one line per workstream (issue → root cause → fix → PR/status), any
   items that still need a human decision, and confirmation that Sentry
   issues were resolved / alerts flagged for manual dismissal as applicable.

## Notes

- This skill's job is triage and dispatch, not fixing anything itself in
  the main session — if you catch yourself reading application code to
  diagnose a specific issue's root cause here, that work belongs in a
  dispatched subagent instead.
- Distinct from `sentry-triage` (files GitHub issues for Sentry errors
  without necessarily fixing them) and `postmortem` (one deep dive on a
  single named incident, with a mandatory detection-gap assessment). This
  skill is the bulk, page-wide sweep that *does* drive fixes to green,
  across whatever mix of issue types the page currently shows.
