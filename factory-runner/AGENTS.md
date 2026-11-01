# factory-runner/ — the Mac-side helper of the software factory

Pairs this Mac with the board and keeps a heartbeat so the board shows it online → [`adr-0041`](../docs/adr-0041-factory-runner-pairing.md). **Separate Go module**, pure Go, no cgo. It doesn't import `api/`: it speaks the Connect protocol's JSON encoding to `RunnerService` (`internal/runner/client.go`), so it carries no generated code.

## Structure

```text
cmd/factory-runner/
  main.go      # app type: pair, run, install, config subcommands (tested)
  config.go    # `config` subcommands
  entry.go     # main(): signals, os.Exit, launchctl (untested wiring)
internal/runner/
  client.go    # Pair, Heartbeat; Connect errors keep their code
  daemon.go    # heartbeat loop: backoff, ends when the token is revoked
  token.go     # device token in the login keychain via `security`
  config.go    # server URL + repo allowlist, owner-only JSON
  validate.go  # job validator
  launchagent.go
```

## Trust

The server is not trusted. The runner's own allowlist (`config repos add owner/name`) and `Validate` decide what runs: a repo outside the allowlist, a branch outside `agent/*`, or any value starting with `-` is refused. Every job must pass `Validate` before anything runs.

- The device token lives only in the keychain; the server keeps its hash. A revoked token ends `run` with `ErrRevoked` — pair again.
- `config server` accepts https only (http for localhost).
- Heartbeat is held open by the server (about 20 s); `Client` times out at 35 s.

## Building

`make build | test | lint | lint/fix`. Tests need no keychain, network or launchd. CI runs lint and test on macOS behind the `changes` filter.
