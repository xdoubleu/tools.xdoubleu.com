# ADR-0042: The runner ships as a signed download and updates itself when idle

- Status: Accepted
- Issues: #2715, #2701
- Affects: `factory-runner/` (`internal/runner/update.go`, `activity.go`,
  `internal/updatesig/`), `.github/workflows/build-factory-runner.yml`,
  `build-api.yml`, `web/components/factory/RunnersClient.tsx`

## Context

The runner is a macOS daemon that runs agent jobs, so a bad binary is code
execution on the owner's Mac. It needs an install path and a way to stay current,
the way `kobo-gateway` has ([adr-0015](adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md)),
without the server being able to push arbitrary code.

## Decision

- **One signed binary, served by the api.** CI builds `factory-runner-darwin-arm64`
  on macOS, signs it on `main` with ed25519 (`kobo-sign`, pointed at the runner's
  own key: secret `FACTORY_RUNNER_SIGNING_KEY`), and stages it into the web build's `downloads/`.
  The public key is compiled in (`FACTORY_RUNNER_UPDATE_PUBKEY`); with none, the
  runner never self-updates.
- **A release file, not an api setting.** `factory-runner-release.txt` holds the
  commit the served binary was built from. The runner and the board both read it,
  so no proto, config or image variable carries the version. It is always this
  commit: no build cache, because a pure-Go build takes seconds.
- **Update only when idle.** Hourly, if no job runs, the runner reads the release
  file; if it differs it downloads the binary and signature beside the heartbeat
  loop (so a slow download never makes it look offline), verifies, checks it is
  Mach-O, re-checks that it is still idle, atomically renames it over itself and
  exits so launchd restarts it. A failure leaves the old binary running; a build
  without the public key skips updating quietly.
- **`install` copies the binary** to `~/Library/Application Support/factory-runner/`
  and runs the LaunchAgent from there, so a download can be deleted and the update
  has one file to replace.
- **Awake only while a job runs.** `Activity` holds `caffeinate -is -w <pid>` while
  any job runs. A closed lid stays awake only on AC power with an external display.

## Consequences

- The signature stops an untrusted server from installing code, but not a replay of
  an older validly signed binary; the release file is unsigned, so the server can
  also withhold updates. Accepted: the key is the owner's, and a rollback needs the
  server to be compromised as well.
- If the release file and the served binary ever disagree, the runner re-downloads
  hourly until they match. CI stages both from one build, so they match by
  construction.
- The board says "update available", not that it will happen: it can't see
  whether the runner has a key or the release was signed.
- A first download is quarantined by Gatekeeper: the install steps clear it with `xattr`.
- Nothing calls `Activity.Begin` yet; the job runner (#2706) does.
