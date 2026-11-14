# factory-runner/ — the Mac-side helper of the software factory

Pairs this Mac with the board and runs jobs → [`adr-0041`](../docs/adr-0041-factory-runner-pairing.md), [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md). **Separate Go module**; cgo only for the menu bar. It speaks Connect JSON to `RunnerService`, never importing `api/`.

## Structure

```text
cmd/factory-runner/   main.go, config.go, entry.go, jobs.go, jobproxy.go
                      menubar_*.go|m|h (menu bar; stub without cgo)
internal/runner/
  client.go, jobs_client.go   the server calls
  daemon.go, control.go       heartbeat loop; pause flag, status
  jobrunner.go                claim loop, `max_jobs`; prompt; Checks:/Blocked:
  jobloop.go, pullstatus.go   a job's attempts until merge; CI verdict
  gitclone.go, hostcheck.go   mirror, clone; host checks
  events.go, publish.go       board log lines; push, open the PR
  harness.go, sandbox.go      Claude Code/OpenCode commands; Sandbox
  sandbox_container.go        one Apple `container` VM per job
  egress_proxy.go, mcp_token.go  the VM's way out; board token
  validate.go, config.go      job validator; server URL, repo allowlist
  token.go, launchagent.go, update.go, activity.go  keychain; plist; update; caffeinate
```

## Trust

The server is untrusted; each job needs UUID ids, an allowlisted repo, an `agent/*` branch, no value starting with `-`, a harness this Mac runs.

- A Reply's job restores the task's pushed branch and resumes its session; only new commits are host-checked.
- The agent owns the clone (it can write `.git/hooks` and `.git/config`); the host runs git there only with `-c core.hooksPath=/dev/null` and a clean config.
- `Sandbox-fix:` branches publish only as pinned `Containerfile`-only PRs ([exception](../docs/convention-unattended-agent-trust.md)).
- Each attempt is host-checked: real commits, no secret. A pass is pushed and opened as a PR → [`adr-0045`](../docs/adr-0045-factory-github-app.md).
- The device token lives only in the keychain.

`jobLoop` resumes the agent's session in one VM with each failure (gate, push, CI) until the PR merges, sharing no state → [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md).

## Distribution → [`adr-0042`](../docs/adr-0042-factory-runner-distribution.md)

`build-factory-runner.yml` signs on `main`; `build-api.yml` stages the dmg, binary, signature and image manifest into `/downloads/`. `-X main.UpdatePublicKey` is the trusted key. 

Settings live in the menu bar; `config …` is for scripting. A fresh pairing is `off`; an older config without a mode becomes `claude`. Log: `~/Library/Logs/factory-runner.log` → [`adr-0049`](../docs/adr-0049-factory-runner-menu-bar.md).

## Building

`make build | dist | test | test/mutation/diff | lint | lint/fix | image`. Tests need no keychain, network, VM or launchd; `-tags container` runs VMs.
