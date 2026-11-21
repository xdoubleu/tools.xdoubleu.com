# factory-runner/ — the Mac-side helper of the software factory

Pairs this Mac with the board and runs jobs → [`adr-0041`](../docs/adr-0041-factory-runner-pairing.md), [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md). **Separate Go module**; cgo only for the menu bar. It speaks Connect JSON to `RunnerService`, never importing `api/`.

## Structure

```text
cmd/factory-runner/   main.go, config.go, entry.go, jobs.go, jobproxy.go
                      menubar_*.go|m|h (menu bar; stub without cgo)
internal/runner/
  client.go, jobs_client.go   the server calls
  daemon.go, control.go       heartbeat loop; pause flag, status
  claimstate.go, watchdog.go  why claims hold (`status`); stall watchdog
  jobrunner.go                claim loop, `max_jobs`; prompt; Checks:/Blocked:/Waiting-on:/Already-fixed:
  sessions.go, jobsweep.go    hourly dir sweep
  jobloop.go, pullstatus.go, gates.go, routines.go  attempts until merge; CI verdict; routines
  gitclone.go, hostcheck.go   clone; host checks
  events.go, publish.go       board log lines; push, open the PR
  vm.go, mcp_token.go         the job VM, proxy and harnesses (../wardcase/vm); board token
  validate.go, config.go      job validator; config; cli_link.go PATH link
  token.go, launchagent.go, update.go  keychain; plist; update
```

## Trust

The server is untrusted; each job needs UUID ids, an allowlisted repo, an `agent/*` branch, no value starting with `-`, a harness this Mac runs.

- A job whose pushed branch exists (Reply or re-run) restores it; only new commits are host-checked.
- The agent owns the clone; the host runs git there only with `-c core.hooksPath=/dev/null` and a clean config.
- `Sandbox-fix:` branches publish only as pinned `Containerfile`-only PRs ([exception](../docs/convention-unattended-agent-trust.md)).
- Each attempt is host-checked: real commits, no secret. A pass is pushed and opened as a PR → [`adr-0045`](../docs/adr-0045-factory-github-app.md).
- The device token lives only in the keychain.

`jobLoop` resumes the agent's session in one VM with each failure (gate, push, CI) until the PR merges, sharing no state → [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md). A run that lost its provider is repeated in place; a block carries its last lines.

## Distribution → [`adr-0042`](../docs/adr-0042-factory-runner-distribution.md)

`build-factory-runner.yml` signs on `main`; `build-api.yml` stages the release files into `/downloads/`. `-X main.UpdatePublicKey` is the trusted key.

`factory-runner status` and the menu show the claim state: claiming, or the one reason it holds. Each minute the watchdog compares the heartbeat's queued-jobs count with it; after 10 minutes of work waiting beside a free slot it logs why and repairs (frees leaked slots, retries the job image, restarts a stuck drain). `lifecycle_test.go` runs the real daemon, claim loop and update drain against fakes; add a scenario for each new stall.

Menu bar and `config …`/`pair` write the same the config file; `install` and the app's first launch link `factory-runner` onto PATH for headless use. Log: `~/Library/Logs/factory-runner.log` → [`adr-0049`](../docs/adr-0049-factory-runner-menu-bar.md).

## Building

Targets, one per line:

- `make build | dist | test | image`
- `make test/cov/diff` (CI's diff gate), `test/mutation/diff`
- `make lint | lint/darwin | lint/fix`: CI lints on macOS, so run `lint/darwin` too; per-OS `syscall` types need `//nolint:gosec,nolintlint`

Tests need no keychain, network, VM or launchd.
