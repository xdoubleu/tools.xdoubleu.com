# Spec: kobo-gateway runtime

- Source of truth: `kobo-gateway/internal/kobogateway/` (`server.go`, `tls.go`, `watcher.go`, `loginitem.go`), `kobo-gateway/cmd/kobo-gateway/` (`menubar_darwin.go`, `recover.go`)
- Issues: —

The loopback-HTTPS and LaunchAgents choices are ADR-0016; the separate module and
toolchain pin are ADR-0015.

## Shape

A menu-bar macOS helper that the books page (`web/`) drives to configure a
USB-mounted Kobo e-reader. The browser makes all authenticated API calls itself
and only hands the gateway a resulting sync URL; the gateway patches the
USB-mounted Kobo's config file directly.

## Behavior

### HTTP server (`server.go`)

Loopback-only HTTPS on `127.0.0.1:41132` (`DefaultPort`): `GET /status`,
`POST /configure`, `POST /revert`, `POST /update`. All requests pass through
`Server.secure()`:

- **Host allowlist** — only `127.0.0.1:<port>` or `localhost:<port>` (blocks DNS
  rebinding).
- **Origin allowlist** — `https://tools.xdoubleu.com` and
  `http://localhost:3000`, plus any `--allow-origin` flags. **The matched
  allowlist string is echoed back, never the raw header**, to avoid request-data
  forgery on the update download URL.
- **CORS + Chrome Private Network Access** — sets
  `Access-Control-Allow-Origin`/`Vary: Origin`, and on an `OPTIONS` preflight
  with `Access-Control-Request-Private-Network: true` also sets
  `Access-Control-Allow-Private-Network: true`. Required because the calling page
  is public HTTPS reaching a private/loopback target.
- POSTs must be `Content-Type: application/json`; `/update` re-validates Origin
  independently before calling `SelfUpdate`.

### TLS (`tls.go`)

`EnsureCert` generates (or loads a persisted) self-signed ECDSA P-256 cert scoped
to `localhost`/`127.0.0.1`, valid 10 years — regenerate by deleting
`cert.pem`/`key.pem`. `EnsureTrusted` runs `security add-trusted-cert` once
(marker file) to add it to the login keychain, prompting the user; if Safari
still rejects it, the fallback is the System keychain (needs sudo). Both live
under `~/Library/Application Support/kobo-gateway`.

### Menu bar (`menubar_darwin.go`)

Runs `macos.RunApp` as an accessory app (no Dock icon) on the main OS thread.
Builds an `NSStatusItem` (icon rendered at **18pt** — an unsized image falls back
to the PNG's native 36px and effectively disappears), a header, an "Open
tools.xdoubleu.com" link, a live Kobo connected/disconnected line, a "Start at
Login" toggle, and Quit.

The header and tooltip (`KoboTooltip`, `watcher.go`) show the release truncated to
7 characters via `kobogateway.ShortRelease` — matching
`web/components/Footer.tsx`'s truncation. The **full SHA** is still what
`gatewayNeedsUpdate` compares exactly.

`watcher.go` polls `FindKobos` and diffs snapshots to emit connect/disconnect
`KoboEvent`s, consumed on the main dispatch queue to update the tooltip/menu line
and fire a best-effort `UNUserNotificationCenter` toast via raw `objc.Call`
(darwinkit has no generated binding for `UserNotifications.framework`). Both
notification paths are gated by `runningInAppBundle` — `UNUserNotificationCenter`
throws with no bundle proxy, e.g. a raw dev binary run outside `KoboGateway.app`.

### Self-update

`POST /update` and the `update` CLI subcommand both call `Updater.SelfUpdate`:
downloads `kobo-gateway-darwin-arm64` from the requesting/configured origin
(size-capped, validated as a Mach-O binary), atomically replaces the running
executable, then **re-signs the `.app` bundle ad-hoc** — overwriting the binary
invalidates the seal `package.sh` applied at build time — and signals a restart.

The restart path relaunches via `open -n <bundle>` inside a real `.app`, **not a
bare `syscall.Exec`**, which would keep the same PID and skip LaunchServices,
leaving `NSStatusItem`/`UNUserNotificationCenter` unregistered after the swap. It
falls back to `syscall.Exec` only for a bundle-less dev binary.

The web UI (`gatewayNeedsUpdate`, `web/lib/books/gatewayClient.ts`) decides *when*
to trigger this by comparing two independent things: the gateway's
`GatewayVersion` against a required floor, and its `release` build stamp against
`getKoboGatewayRelease()` (see ADR-0004). `'dev'` on either side skips the check.

### Crash recovery (`recover.go`)

Defense-in-depth on top of launchd's `KeepAlive`:

- `guard`/`recoverGo` recover and report a panic in a single event/block (a
  dispatched menu update, a notification call) to Sentry and **swallow** it, so
  one bad event doesn't take the whole app down.
- `reportAndRepanic` (deferred once in `main`) reports then **re-panics** —
  swallowing a main-thread panic would leave the app running in a broken,
  un-relaunched state, whereas re-panicking exits non-zero and lets `KeepAlive`
  relaunch a fresh process.
- `reportFatal` covers a plain error `run()` returns (not a panic, e.g. a bind
  failure) — that otherwise only goes to stderr, invisible on a console-less
  menu-bar app.

Crash reporting goes to Sentry (`initSentry` in `main.go`), gated on a build-time
`-ldflags -X main.SentryDSN=...` var — empty in dev builds and `go test`, so
nothing is ever sent off a dev machine.

## Invariants

- **The status item, its button, and its live line are package-level vars, not
  locals.** `objc.Retain` installs a Go finalizer that releases the object once
  its Go wrapper is GC'd, so a value that only lives in a setup closure would have
  its icon disappear a few GC cycles after launch.
- `buildStatusItem` is called once at startup **and again** from an
  `NSWorkspaceDidWakeNotification` observer — macOS can silently drop a status
  item's on-screen presence across sleep/wake even though the retained object
  stays alive.
- Never echo the raw `Origin` header back; echo the matched allowlist entry.
- `REQUIRED_GATEWAY_VERSION`/`GatewayVersion` is a floor for **genuine protocol
  breaks only**; bump both together and only then.

## Known gaps

- **None of the recovery paths catch darwinkit's `SIGABRT`** — a native abort that
  bypasses Go's panic machinery entirely. That stays covered by launchd
  `KeepAlive` alone.
- `codecov.yml` excludes `menubar_darwin.go` from coverage: pure AppKit/cgo with
  no window-server session under `go test`.
