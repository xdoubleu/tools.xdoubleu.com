# ADR-0045: A GitHub App mints each job's push token; the Mac holds no GitHub credential

- Status: Accepted (the plan of #2775, decision 13 of #2701)
- Issues: #2775, #2701
- Affects: `api/internal/githubapp/`, `RunnerService.GetPushToken`,
  `CompleteJobRequest.pr_url`, `factory-runner`

## Context

The agent in the sandbox is assumed hostile, so every GitHub write stays on the
host, after the host's own checks ([adr-0043](adr-0043-factory-runner-sandbox.md)).
A personal access token would be long-lived, broad and kept on the Mac, and a
deploy key can push but can't open a pull request.

## Decision

- **A dedicated "factory" GitHub App**, installed on the repos the factory works
  on. Its id and PEM key are deploy secrets (`FACTORY_GITHUB_APP_ID`,
  `FACTORY_GITHUB_APP_PRIVATE_KEY`, in the three places
  [convention-deploy-secrets](convention-deploy-secrets.md) asks for). Unset or
  unparsable, nothing can push and a job ends in Needs you with its work on the
  Mac, as before.
- **A token per job, narrowed.** The server signs a short App JWT, finds the
  installation of the job's repo and asks for an installation token limited to
  that repo, with `contents` and `pull_requests` write and nothing else: no
  workflows, issues or administration. It lives under an hour.
- **The runner asks for it, only for its own running job.** `GetPushToken` needs
  the device token, a job in `running` and that the runner claimed it (a job
  records its runner; so does `CompleteJob`). The runner calls it after its own
  checks pass, and the sandbox never sees it. GitHub's own error text never
  reaches the runner; a failure there is `Unavailable`, to retry.
- **The server checks what the runner reports.** A pull request link is kept only
  if it is `https://github.com/<owner>/<repo>/pull/<n>` of the job's own repo, for
  a failed job too so no open PR is orphaned. A succeeded job with one moves its
  task to In review; every other job goes to Needs you.
- **Status is polled, with a read-only token.** Every five minutes a job reads
  each open pull request of a job (`pull_requests` and `checks` read, minted per
  repo and reused until shortly before it expires) and keeps its state, a CI
  rollup and the standing review on the job. No public webhook endpoint is needed.
  A merge moves a task that is still In review to Done and a close without a merge
  to Needs you; a task a person has moved on, or one a newer job is on, is left.
- **Never auto-merge.** The App may open a PR but nothing here merges one.
  `main`'s ruleset must not list the App as a bypass actor.

## Consequences

- Revoking the App's installation stops all pushing at once.
- The installation id is remembered per repo and forgotten on any failure to
  mint a token, so a reinstall needs no restart.
- The runner pushes the host-owned review copy, never the agent's clone, with the
  token in git's environment (never argv), never forces, and opens the PR (or finds
  the branch's open one). Any failure there leaves the work on the Mac and the task
  in Needs you, with the reason ahead of the agent's own text. The push ignores
  the Mac's own git config (a developer's `insteadOf` would push over SSH as them),
  a transient token or PR failure is retried, and mentions and closing keywords in
  the agent's text are defused in the PR body.

## Revisit when

A job needs more than one repo, or GitHub offers a narrower permission for
opening a PR than `pull_requests` write.
