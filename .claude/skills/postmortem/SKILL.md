---
name: postmortem
description: Take one production incident (a Sentry issue, a failing job, a user-reported symptom) and go deep — establish root cause, then answer "which tool or alert should have caught this and didn't" and fix that gap first. Use whenever the user asks "what went wrong with X", "postmortem this", "why did X break", "root cause this", or "how did we not catch this". Distinct from `sentry-triage`, which is a bulk sweep of all unresolved Sentry issues rather than a deep dive on one incident.
---

# Postmortem

Root cause plus detection: executes
[`convention-mcp-gap-first`](../../../docs/convention-mcp-gap-first.md) for
one incident. Read-mostly plus issue filing — no code fixes; those go through
`start-task`/`finish-task`.

## Steps

1. **Take the incident** (Sentry issue, failed job, or symptom). If only a
   symptom and the app isn't obvious, ask which — don't read the wrong app's
   logs.

2. **Gather evidence** via MCP: `get_logs` (source/level/since),
   `get_job_stats`, `prom_query` (host, Postgres, `/metrics` latency
   histograms), `get_grafana_alerts`, `get_sentry_issues`,
   `get_slow_transactions`, `get_database_stats`, `get_workflow_runs`.
   Delegate the noisy pulls (`get_logs`, `get_sentry_issues`) to a subagent
   that returns distilled findings.

3. **Root cause** as `file:line`, not a paraphrase of the error (`ast-grep`,
   then go-to-definition/find-references).

4. **Assess detection** — exactly one:
   - **No gap** — a tool/alert showed it and it was acted on.
   - **Alerting gap** — data was visible but nothing surfaced it proactively.
   - **Latency gap** — surfaced too late to act before impact.
   - **Coverage gap** — invisible, or a tool returned wrong/incomplete data.

   Don't invent a gap; but call out "no gap" explicitly, since it's rare.

5. **File the detection-gap issue first** via `refine-issue`, separate from
   the fix issue so it survives the fix shipping. Skip if "no gap".

6. **File the root-cause fix issue** via `refine-issue` (priority per
   `.claude/github-triage.config.json` — a live incident is almost always P0).

7. **Record the gap** in `docs/convention-mcp-gap-first.md`'s gap list once
   its fix is known, matching the existing entry format.

8. **Report:** root cause (`file:line`), detection verdict, issue links.

If the incident is a Sentry issue already tracked by an open GitHub issue,
don't refile — comment the detection assessment there, and still do step 7.
