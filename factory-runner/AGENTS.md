# factory-runner/ — the Mac-side helper of the software factory

Pairs this Mac with the board, heartbeats and runs jobs → [`adr-0041`](../docs/adr-0041-factory-runner-pairing.md), [`adr-0044`](../docs/adr-0044-factory-jobs-queue.md). **Separate Go module**, pure Go, no cgo. It doesn't import `api/`: it speaks Connect's JSON to `RunnerService` (`internal/runner/client.go`), so no generated code.

## Structure

```text
cmd/factory-runner/   main.go (the tested app type), config.go, entry.go (untested wiring)
internal/runner/
  client.go, jobs_client.go   the server calls
  daemon.go                   heartbeat loop, backoff
  jobrunner.go                claim loop: validate, clone, sandbox, report
  gitclone.go                 host mirror and a standalone job clone
  hostcheck.go                host checks on finished work
  events.go                   redacted log lines to the board
  publish.go                  push the reviewed branch, open its PR
  harness.go, sandbox.go      Claude Code's command; the Sandbox interface
  validate.go, config.go      job validator; server URL and repo allowlist
  token.go, launchagent.go    keychain; plist
  update.go, activity.go      signed self-update; caffeinate
internal/updatesig/           copy of kobo-gateway's (Go forbids importing its internal/)
```

## Trust

The server is not trusted: the runner checks a claimed job itself: a UUID id (it names a directory), the local repo allowlist (`config repos add owner/name`), an `agent/*` branch, no value starting with `-`, and a harness it can run.

- A job's clone is the agent's once the sandbox starts: it can write `.git/hooks` and `.git/config`, so the host never runs git in it without `-c core.hooksPath=/dev/null` and a clean config; it reads the outcome as a file.
- A finished job is host-checked before it counts: real commits, no workflow change, no secret in commits or outcome, read from a fetched copy. A pass is pushed and opened as a PR → [`adr-0045`](../docs/adr-0045-factory-github-app.md).
- The device token lives only in the keychain; a revoked one ends `run` with `ErrRevoked`.
- `config server` takes https only (http for localhost). `Client` times out at 35 s, above the server's 20 s holds.

## Distribution → [`adr-0042`](../docs/adr-0042-factory-runner-distribution.md)

`build-factory-runner.yml` signs on `main` (`FACTORY_RUNNER_SIGNING_KEY`); `build-api.yml` stages binary, `.sig` and `factory-runner-release.txt` into `/downloads/`. `-X main.UpdatePublicKey` is the trusted key (empty: no self-update, which waits for idle `Activity`).

## Building

`make build | test | lint | lint/fix`. Tests need no keychain, network, VM or launchd; CI runs on macOS.
