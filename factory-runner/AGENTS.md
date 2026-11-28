# factory-runner/ — the Mac-side helper of the software factory

Pairs this Mac with the board, runs jobs, chats → [`adr-0041`](../docs/adr-0041-factory-runner-pairing.md), [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md), [`adr-0061`](../docs/adr-0061-factory-chat-runner-bridge.md). **Separate Go module**; cgo only for the menu bar. It speaks Connect JSON to `RunnerService`, never importing `api/`.

## Structure

```text
cmd/factory-runner/   main.go, config.go, entry.go, jobs.go, jobproxy.go
                      menubar_*.go|m|h (menu bar; stub without cgo)
internal/runner/
  client.go, jobs_client.go   the server calls
  daemon.go, control.go       heartbeat loop; pause flag, status
  claimstate.go, watchdog.go  why claims hold (`status`); stall watchdog
  envfault.go                 environment breaker, preflight before each claim
  jobrunner.go                claim loop, `max_jobs`; prompt; Checks:/Blocked:/Waiting-on:/Already-fixed:
  sessions.go, jobsweep.go    hourly dir sweep
  attemptsession.go           Claude attempts via wardcase
  jobloop.go, pullstatus.go, gates.go, routines.go  attempts until merge; CI verdict; routines
  gitclone.go, hostcheck.go   clone; host checks
  events.go, publish.go       board log lines; push, open the PR
  vm.go, mcp_token.go         the job VM, proxy and harnesses (../wardcase/vm); board token
  validate.go, config.go      job validator; config; cli_link.go PATH link
  token.go, launchagent.go, update.go  keychain; plist; update
```

## Trust

The server is untrusted; each job needs UUID ids, an allowlisted repo, an `agent/*` branch, no value starting with `-`, a harness this Mac runs.

- A job whose pushed branch exists restores it; only new commits are host-checked.
- The agent owns the clone; host git there runs with `-c core.hooksPath=/dev/null` and a clean config.
- `Sandbox-fix:` branches publish only as flake-only PRs ([exception](../docs/convention-unattended-agent-trust.md)).
- Each attempt is host-checked: real commits, no secret. A pass is pushed and opened as a PR → [`adr-0045`](../docs/adr-0045-factory-github-app.md).
- The device token lives only in the keychain.

A Claude job's attempts run through `wardcase/session` on the job's clone: `on-limit fallback` is `OverflowAuto` on `fallback-model`; `wait` or a Reply is `OverflowOff` and the limited job requeues. The OpenRouter key stays on the host; tests fake `SessionAPI`.

`jobLoop` resumes the agent's session in one VM with each failure (gate, push, CI) until the PR merges, sharing no state → [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md). A run that lost its provider is repeated in place.

An attempt that dies on the runner's own environment before the agent starts (a dead Nix mount) ends `limited` with a stable `Blocked:` line: the task is requeued untouched and the server files one `Sandbox:` task per cause. The breaker then holds claims (the `runner fault` reason in `status` and the menu) until `Preflight` passes, probed every 30s and before every claim.

## Distribution → [`adr-0042`](../docs/adr-0042-factory-runner-distribution.md)

`build-factory-runner.yml` signs on `main`; `build-api.yml` stages release files into `/downloads/`; `-X main.UpdatePublicKey` is the trusted key.

`factory-runner status` and the menu show the claim state, or the one reason it holds. After 10 minutes of queued work beside a free slot the watchdog logs why and repairs it. `lifecycle_test.go` runs the real daemon against fakes; add a scenario for each new stall.

`on-limit fallback` needs `fallback-model <id>` (`none` clears). Menu bar and `config …`/`pair` write one config file; `install` and first launch link `factory-runner` onto PATH. Log `~/Library/Logs/factory-runner.log` → [`adr-0049`](../docs/adr-0049-factory-runner-menu-bar.md).

## Building

Targets, one per line:

- `make build | dist | test | image`
- `make test/cov/diff` (CI's diff gate), `test/mutation/diff`
- `make lint | lint/darwin | lint/fix`: CI lints on macOS, so run `lint/darwin` too

Tests need no keychain, network or VM.
