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
- **The runner asks for it, only for a running job.** `GetPushToken` needs the
  device token and a job in `running`; the runner calls it after its own checks
  pass, and the sandbox never sees it.
- **The server checks what the runner reports.** A pull request link is kept only
  for a succeeded job and only if it is `https://github.com/<owner>/<repo>/pull/<n>`
  of the job's own repo; the task then moves to In review, otherwise to Needs you.
- **Never auto-merge.** The App may open a PR but nothing here merges one.
  `main`'s ruleset must not list the App as a bypass actor.

## Consequences

- Revoking the App's installation stops all pushing at once.
- The installation id is remembered per repo and forgotten when GitHub says it
  is gone, so a reinstall needs no restart.
- The runner's half (push from the host-owned review copy with the token in an
  environment-supplied header, never argv; open the PR) lands separately.

## Revisit when

A job needs more than one repo, or GitHub offers a narrower permission for
opening a PR than `pull_requests` write.
