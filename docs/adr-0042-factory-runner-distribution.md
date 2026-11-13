# ADR-0042: The runner ships as a signed download and updates itself when idle

- Status: Accepted
- Issues: #2715, #2701
- Affects: `factory-runner/` (`internal/runner/update.go`, `activity.go`,
  `../selfupdate/`, `cmd/factory-runner/assets/`), `.github/workflows/build-factory-runner.yml`,
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
- **A drag-to-Applications dmg for first install.** `make dist` also packages
  `FactoryRunner.app` into `factory-runner.dmg`; opening the app runs `install`
  (copy the binary, write and load the LaunchAgent) and opens the board's Runner
  page to pair. The app bundle is ad-hoc signed for Gatekeeper; the ed25519
  signature above guards the self-update binary, not the dmg.
- **A release file, not an api setting.** `factory-runner-release.txt` holds the
  commit the served binary was built from. The runner and the board both read it,
  so no proto, config or image variable carries the version. It is always this
  commit: no build cache, because a pure-Go build takes seconds.
- **A versioned job image rides the same release.** The job image (adr-0043) is
  built on `main` for the release, tagged `factory-job:<release>`, and staged as an
  OCI archive plus a signed manifest into `downloads/`. The runner ensures it is
  present — fetching, verifying the ed25519 signature against its compiled-in key,
  checking the release and a sha256, and `container image load`-ing it — at start and
  after each self-update, so the image's pinned tool versions change only with a
  runner release.
- **Update only when idle.** Hourly, if no job runs, the runner reads the release
  file; if it differs it downloads the binary and signature beside the heartbeat
  loop (so a slow download never makes it look offline), verifies, checks it is
  Mach-O, re-checks that it is still idle, atomically renames it over itself and
  exits so launchd restarts it. A failure leaves the old binary running. Every
  check logs its outcome (newer release installed, none, failed, or no key in
  the build) to the LaunchAgent log and the menu bar → [adr-0049](adr-0049-factory-runner-menu-bar.md).
- **`install` copies the app** (a bare binary gets a minimal bundle) to
  `~/Library/Application Support/factory-runner/FactoryRunner.app` and runs the
  LaunchAgent from the binary inside, so a download can be deleted and the update has
  one file to replace. It has to be a bundle for macOS to show the runner's
  notifications, so install and self-update re-seal it with an ad-hoc signature.
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
- A first download is quarantined by Gatekeeper: the dmg's app needs a right-click
  -> Open once; terminal installs clear it with `xattr`.
- Nothing calls `Activity.Begin` yet; the job runner (#2706) does.
