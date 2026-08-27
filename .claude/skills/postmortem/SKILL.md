---
name: postmortem
description: Take one production incident (a Sentry issue, a failing job, a user-reported symptom) and go deep — establish root cause, then answer "which tool or alert should have caught this and didn't" and fix that gap first. Use whenever the user asks "what went wrong with X", "postmortem this", "why did X break", "root cause this", or "how did we not catch this". Distinct from `sentry-triage`, which is a bulk sweep of all unresolved Sentry issues rather than a deep dive on one incident.
---

# Postmortem

Root cause is only half of a postmortem. Root `CLAUDE.md`'s "MCP coverage
gaps" section already states the rule this skill executes: if there's no
tool that surfaces a production issue, or an existing tool returns
wrong/incomplete data, fix that gap first — otherwise the same blind spot
just recurs next time. Three worked examples are documented there (#1027
egress bytes, #1195 OAuth scopes, #1214 notification settings); this skill
is what turns that convention into a step that actually runs, for any new
incident.

Reads across the observability MCP tools (`mcp__tools-apps__get_logs`,
`get_job_stats`, `get_host_metrics`, `get_sentry_issues`,
`get_slow_transactions`, `get_database_stats`, `get_workflow_run_stats`) and
uses `refine-issue` to file the resulting issue(s).

## Steps

1. **Take the incident.** A Sentry issue, a failed job run, or a symptom the
   user describes. If the user only names a symptom, ask which system/app it
   showed up in if it isn't obvious — don't guess the wrong app's logs.

2. **Gather evidence** across the observability MCP tools rather than
   guessing at the cause: `get_logs` (filtered by source/level/since),
   `get_job_stats`, `get_host_metrics`, `get_sentry_issues`,
   `get_slow_transactions`, `get_database_stats`, `get_workflow_run_stats`.
   Per root `CLAUDE.md`'s "Delegating to Subagents" section, delegate the
   noisy pulls (`get_logs`, `get_sentry_issues`) to a subagent and have it
   return only the distilled findings — don't pull raw log/issue output into
   the main context yourself.

3. **Establish root cause** in this codebase's terms — `file:line`, not a
   paraphrase of the error message. Use `ast-grep` to find the shape of the
   failing code and the LSP tool's go-to-definition/find-references once a
   concrete symbol is in hand, per the Code Navigation rules in root
   `CLAUDE.md`.

4. **Assess detection.** The step that distinguishes this skill from a plain
   bug investigation. Classify the incident honestly into exactly one of:
   - **No gap** — a tool or alert showed it, and it was acted on.
   - **Alerting gap** — a tool showed the data, but nothing surfaced it
     proactively; it was only visible if someone went looking.
   - **Latency gap** — a tool showed it, but too late to act before impact.
   - **Coverage gap** — it was invisible: no tool surfaced it at all, or an
     existing tool returned wrong/incomplete data.

   "No gap found" is a legitimate outcome — don't invent a gap to satisfy
   this step. But it should be rare enough to call out explicitly when it
   happens, since most incidents that reach this skill got here because
   nobody caught them proactively.

5. **File the detection-gap issue first**, via `refine-issue`, separate from
   the root-cause fix issue. Keeping them separate means the detection half
   doesn't silently get dropped once the code fix ships and the issue is
   closed — the gap is its own trackable deliverable (a new/corrected MCP
   tool, a new alert, etc). Skip this step entirely if step 4 concluded "no
   gap."

6. **File the root-cause fix issue** via `refine-issue`, applying the
   priority rule from `.claude/github-triage.config.json` — a live incident
   almost always resolves to P0 (something that already works is now broken
   in production).

7. **Append the gap and its fix to root `CLAUDE.md`'s "MCP coverage gaps"
   list**, once the detection-gap issue's fix is known, in the same
   `- *Issue #NNNN* —` format as the three existing entries. That section
   explicitly asks for this ("Note the gap and fix in this file when it
   happens"); do it here rather than leaving it to be done by hand later.

8. **Report** a short summary: root cause (file:line), detection verdict,
   and the issue(s) filed with links.

## Notes

- This skill is read-mostly plus issue filing — it does not apply code
  fixes itself. Once the fix issue is filed, fixing it goes through the
  normal `start-task`/`finish-task` flow like any other change.
- Distinct from `sentry-triage`: that skill sweeps *all* unresolved Sentry
  issues in bulk and files a tracking issue per one, with no detection-gap
  step. This skill takes *one* incident (which may not even be a Sentry
  issue — a failing job or a user-reported symptom both qualify) and goes
  deep, with the detection assessment as the step that makes it a
  postmortem rather than just a bug report.
- If the incident is itself a Sentry issue already tracked by an open
  GitHub issue, don't refile — add the detection assessment as a comment on
  the existing issue instead, and still check whether root `CLAUDE.md`
  needs an entry.
