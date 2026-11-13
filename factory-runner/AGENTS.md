# factory-runner/ — the Mac-side helper of the software factory

Pairs this Mac with the board; runs jobs → [`adr-0041`](../docs/adr-0041-factory-runner-pairing.md), [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md). **Separate Go module**; cgo only for the menu bar. It speaks Connect's JSON to `RunnerService`, never importing `api/`.

## Structure

```text
cmd/factory-runner/   main.go (the tested app type), config.go, entry.go, jobs.go, jobproxy.go (claim loop, job proxy)
                      menubar_*.go|m|h (menu bar; stub without cgo)
internal/runner/
  client.go, jobs_client.go   the server calls
  daemon.go, control.go       heartbeat loop; pause flag, menu status
  jobrunner.go                claim loop; Checks:/Blocked: lines
  gitclone.go                 host mirror and a standalone job clone
  hostcheck.go                host checks on finished work
  events.go                   redacted log lines to the board
  publish.go                  push the reviewed branch, open its PR
  harness.go, sandbox.go      Claude Code and OpenCode commands; Sandbox
  sandbox_container.go        one Apple `container` VM per job
  egress_proxy.go, mcp_token.go  the VM's only way out; the job's board token
  validate.go, config.go      job validator; server URL and repo allowlist
  token.go, launchagent.go    keychain; plist
  update.go, activity.go      signed self-update; caffeinate
```

## Trust

The server is untrusted; each claimed job is checked for UUID ids, the local repo allowlist, an `agent/*` branch, no value starting with `-`, a harness it can run.

- A Reply's job restores the task's pushed branch and resumes its session; only new commits are host-checked.
- A job's clone is the agent's once the sandbox starts: it can write `.git/hooks` and `.git/config`, so the host runs git there only with `-c core.hooksPath=/dev/null` and a clean config.
- A finished job is host-checked: real commits, no workflow change, no secret in commits or outcome. A pass is pushed and opened as a PR → [`adr-0045`](../docs/adr-0045-factory-github-app.md).
- The device token lives only in the keychain.

## Distribution → [`adr-0042`](../docs/adr-0042-factory-runner-distribution.md)

`build-factory-runner.yml` signs on `main`; `build-api.yml` stages the dmg, binary, signature and release file into `/downloads/`. The dmg's app installs a bundle copy and LaunchAgent; `-X main.UpdatePublicKey` is the trusted key. Shared: `../selfupdate`.

Menu bar Stop writes a `paused` flag; logs go to `~/Library/Logs/factory-runner.log` → [`adr-0049`](../docs/adr-0049-factory-runner-menu-bar.md).

## Building

`make build | dist | test | lint | lint/fix | image`. Tests need no keychain, network, VM or launchd; `-tags container` runs real VMs.
