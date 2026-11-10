# ADR-0049: The runner has a menu bar icon; Stop is a persisted pause flag

- Status: Accepted
- Affects: `factory-runner/cmd/factory-runner/menubar_*`, `internal/runner/control.go`,
  `daemon.go`, `launchagent.go`
- Builds on: [adr-0036](adr-0036-kobo-gateway-native-appkit-menu-bar.md),
  [adr-0042](adr-0042-factory-runner-distribution.md)

## Context

The runner was a headless LaunchAgent: no way to see its state or stop it
without a terminal or the board. Self-update looked broken, but it worked: it
ran hourly and installed releases. It just logged nothing on success or "no
newer release", and the LaunchAgent had no log path, so stderr went nowhere.

## Decision

- **Menu bar icon, as in kobo-gateway**: hand-written Objective-C behind cgo,
  main thread only, a stub for `!darwin || !cgo`; `make build` forces
  `CGO_ENABLED=1`. The status lines refresh whenever the menu opens.
- **Stop is a pause flag file** (`paused` in the install directory), not an
  unloaded LaunchAgent. The process keeps running, so the icon stays and Start
  needs no terminal; the flag survives launchd restarts and reboots. While
  paused the loop claims no jobs, the heartbeat goes silent (the board shows the
  runner offline after its usual timeout) and update checks are skipped.
- **A running job is never killed by Stop.** The heartbeat continues until the
  job ends; the menu reads "Stopping after the current job".
- **Quit unloads the LaunchAgent** (`launchctl bootout`), so launchd doesn't
  start the runner again until the next login.
- **Update checks are visible**: each logs its outcome and the menu shows the
  latest; "Check for updates" requests one now (still only when idle).
- **Notifications, shared with kobo-gateway** (`selfupdate/notification`): a manual
  update check toasts "Checking…" and its outcome, a refused or deferred check says
  why, and an installed update always toasts; the hourly check is silent. Both apps
  share `appbundle` and `notification` through `../selfupdate`; outside a bundle,
  posting is a no-op.
- **Both menu bars match**: status lines, then actions, "Start at Login" and Quit;
  neither opens a URL. kobo-gateway gained "Check for updates" (it compares the
  published `kobo-gateway-release.txt` with its own release before downloading).
- **Start at Login is the LaunchAgent plist**: on writes it, off removes it. The
  running process and loaded agent are untouched, so the runner keeps going
  until Quit or logout.
- **"Check for updates" acts at once**: it wakes the loop out of the held
  heartbeat, and the menu says when it was refused (stopped) or is waiting
  (a job is running).
- **A circle-and-play glyph**, distinct from kobo-gateway's e-reader.
- **The LaunchAgent logs** to `~/Library/Logs/factory-runner.log`. An already
  installed agent gets the path on the next `install`.

## Consequences

- A paused runner is offline on the board but not distinguishable there from a
  crashed one; the menu is the only place that says "Stopped".
