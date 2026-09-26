# Spec: agent routines on GitHub Actions

- Status: all four routines run on Actions; the claude.ai routines are retired
- Issues: #1902, #1903, part of #1851. Depends on #1899 (intake hardening),
  #1901 (machine identity). Grafana-fired runs: #1907.

Each routine is a thin caller workflow (`routine-<name>.yml`, `schedule:` +
`workflow_dispatch:` only) around `.github/workflows/agent-routine.yml`. That
workflow:

1. mints a one-hour GitHub App token and a client_credentials MCP token
   ([ADR-0025](adr-0025-machine-client-credentials-service-role.md)),
2. opens the run's `automated_actions` row through `record_action`
   (`scripts/routine_record.sh`; `trigger_source` `schedule`, `manual`, or `ci`
   for a red `main`),
3. runs `opencode run --auto` on OpenRouter with the routine's prompt, writing
   the transcript to a file rather than the public job log,
4. preflights the apps MCP server (`scripts/routine_preflight.sh`): fails the
   job before the agent starts if the required tools aren't exposed,
5. runs `scripts/routine_watchdog.sh` beside the agent; it kills opencode and
   fails the step if the agent loops on the same tool call (a degenerate loop
   would otherwise spin until the step timeout since every call "succeeds"),
6. measures the run from the transcript (`scripts/routine_metrics.sh`: requests,
   tokens, estimated cost, duration, tool calls, failed and repeated calls)
   into the job summary,
7. closes the row with the agent's outcome file (`failed` if the agent step
   didn't succeed or wrote none) and those metrics,
8. uploads the transcript encrypted,
9. always posts a Slack notice: the job outcome, the agent's summary, the
   metrics line, and the run link.

The collector exports each routine's latest measured run as
`automated_action_last_run{routine,metric}`; `AutomatedRoutineRunHeavy` flags a
run with over 3x its 14-day average requests.

## Feedback issues

The agent ends each run with a retro. It reports blockers and concrete waste
from that run as `routine-feedback` issues, commenting on an open match
instead of duplicating, at most 2 per run. They never go to Ready: the owner
triages them like any Backlog issue. They hold no log excerpts or copied
input text, since the inputs are untrusted.

A red `main` also starts red-PR repair: `main.yml`'s `notify-main-ci-red` job
calls `agent-routine.yml` directly.

Rules the routines follow: [convention-unattended-agent-trust](convention-unattended-agent-trust.md).

## One-time setup

1. **GitHub App** (Settings → Developer settings → GitHub Apps → New).
   Private, webhook off, installed only on this repo. Repository permissions:
   - Contents, Pull requests, Issues: read and write.
   - Actions, Checks, Commit statuses: read.
   - Metadata: read.
   - **Not** Workflows, so the bot can't push `.github/workflows/` changes.

   Apps can't reach user-owned Projects, so board reads go through
   `get_project_issues_by_status`. Status writes fail, and the routine reports
   that.
2. **Environment `agents`** (Settings → Environments), with deployment branches
   limited to `main`. Give `production` the same limit, so a workflow on a bot
   branch can read neither environment. Secrets:
   - `ROUTINES_APP_PRIVATE_KEY`: the App's private key (.pem contents).
   - `OPENROUTER_API_KEY`: a key with a credit limit, since the agent's shell
     can read it.
   - `OAUTH_ROUTINES_CLIENT_SECRET`: the same value as the deploy secret.
   - `SLACK_WEBHOOK_URL`: a Slack Incoming Webhook the routine steps post
     their outcome notice to.
   - `ROUTINE_TRANSCRIPT_PASSPHRASE`: `openssl rand -hex 32`. Without it, no
     transcript is uploaded.
   - `POSTHOG_MCP_API_KEY`: a PostHog personal API key with read access to the
     project, for the PostHog UX discovery routine only.
3. **Repository variables**:
   - `ROUTINES_APP_ID`.
   - `ROUTINES_MODEL`: an OpenRouter model id without the `openrouter/`
     prefix, e.g. a GLM or DeepSeek flash model.
   - `POSTHOG_MCP_URL`: PostHog's remote MCP endpoint for your region, from
     PostHog's MCP docs.
   - `ROUTINE_<NAME>_ENABLED=true` per routine: `RED_PR_REPAIR`, `NIGHTLY_MAINTENANCE_SWEEP`,
     `READY_ISSUES_EXECUTOR`, `POSTHOG_UX_DISCOVERY`. These must be
     repo-level: the callers' `if:` runs outside the `agents` environment.
     `workflow_dispatch` runs regardless.

## Reading a transcript

Download the run's artifact, then:

```bash
openssl enc -d -aes-256-cbc -pbkdf2 -pass pass:"$ROUTINE_TRANSCRIPT_PASSPHRASE" \
  -in transcript.tar.gz.enc | tar -xzf -
```

`transcript.jsonl` holds OpenCode's raw events (`--format json`).

## Constraints

- The MCP token lasts an hour, so the agent step is capped at 55 minutes,
  leaving time to close the run record.
- Metric cost is OpenCode's estimate from its model catalog, not the
  OpenRouter bill. Requests count the main session's model calls only.
- OpenCode subagents share one checkout, so routines work through items in
  sequence rather than in parallel isolated subagents. The ready-issues
  executor takes at most 2 issues per run to fit the cap.
- `.opencode/plugins/repo-guard` does nothing under `GITHUB_ACTIONS`, because
  the runner's checkout is throwaway.
- The agent's shell holds the App, MCP and OpenRouter credentials. The bash
  deny-list (force-push, `gh pr merge`, `gh secret`) is defence in depth, not
  a boundary. The boundaries are the trust convention, short-lived tokens,
  the App's permissions, and human merges.
