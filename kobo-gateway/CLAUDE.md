# kobo-gateway/ — kobo-gateway macOS app

A menu-bar macOS helper that the books page (`web/`) drives to configure a
USB-mounted Kobo e-reader. It's a **separate Go module** (`module
tools.xdoubleu.com/kobo-gateway`, its own `go.sum`) because its menu bar needs cgo
and the real AppKit/Xcode SDK ([`darwinkit`](https://github.com/progrium/darwinkit))
— pulling that into the root/`api` module would force every Linux
build/lint/test of the main server to carry an Objective-C dependency it
never uses.

## Structure

```text
kobo-gateway/
├── cmd/kobo-gateway/
│   ├── main.go            # entrypoint: flags, Sentry init, TLS bootstrap, self-update restart loop
│   ├── bundle.go          # runningInAppBundle() — detects a real .app vs a raw dev binary
│   ├── menubar_darwin.go  # //go:build darwin — real AppKit status-bar item
│   ├── menubar_other.go   # //go:build !darwin — no-op stub, blocks on <-stop (keeps Linux/CI green)
│   ├── notify.go          # platform-agnostic notify func var, wired to AppKit by menubar_darwin's init()
│   ├── recover.go         # panic recovery + Sentry reporting helpers
│   └── assets/            # Info.plist template, icon PNGs, package.sh (bundles the .app/.dmg)
└── internal/kobogateway/  # loopback HTTPS server: routes, security, TLS, conf/device/update logic
```

`internal/kobogateway` is plain Go with no AppKit dependency — it's the part
that actually talks to the Kobo and the browser. Setup is gateway-only: the
browser never reads the Kobo's conf file itself, so `conf.go` is the only
place that parses/serializes `Kobo eReader.conf` (mirrors
`web/lib/books/koboConf.ts`, which only keeps simple string checks).

## HTTP Server (`internal/kobogateway/server.go`)

Loopback-only HTTPS on `127.0.0.1:41132` (`DefaultPort`): `GET /status`,
`POST /configure`, `POST /revert`, `POST /update`. All requests pass through
`Server.secure()`:

- **Host allowlist** — only `127.0.0.1:<port>` or `localhost:<port>` (blocks DNS rebinding).
- **Origin allowlist** — `https://tools.xdoubleu.com` and `http://localhost:3000` (plus any `--allow-origin` flags); the matched allowlist string is echoed back, never the raw header, to avoid request-data forgery on the update download URL.
- **CORS + Chrome Private Network Access** — sets `Access-Control-Allow-Origin`/`Vary: Origin`, and on an `OPTIONS` preflight with `Access-Control-Request-Private-Network: true` also sets `Access-Control-Allow-Private-Network: true` — required because the calling page is public HTTPS reaching a private/loopback target.
- POSTs must be `Content-Type: application/json`; `/update` re-validates Origin independently before calling `SelfUpdate`.

HTTPS (not HTTP) is required because Safari blocks a secure page (the books
page is always `https://`) from fetching a plain-HTTP loopback URL — Chrome
exempts loopback from that check, Safari doesn't.

## TLS (`internal/kobogateway/tls.go`)

`EnsureCert` generates (or loads a persisted) self-signed ECDSA P-256 cert
scoped to `localhost`/`127.0.0.1`, valid 10 years — regenerate by deleting
`cert.pem`/`key.pem`. `EnsureTrusted` runs `security add-trusted-cert` once
(marker file) to add it to the login keychain, prompting the user; if Safari
still rejects it, the fallback is the System keychain (needs sudo). Both
live under `~/Library/Application Support/kobo-gateway`.

## Menu Bar (`menubar_darwin.go`)

Runs `macos.RunApp` as an accessory app (no Dock icon) on the main OS thread.
Builds an `NSStatusItem` (icon rendered at 18pt — an unsized image falls back
to the PNG's native 36px and effectively disappears), a header, an "Open
tools.xdoubleu.com" link, a live Kobo connected/disconnected line, a "Start
at Login" toggle, and Quit.

The status item, its button, and its live line are package-level vars, not
locals — `objc.Retain` installs a Go finalizer that releases the object once
its Go wrapper is GC'd, so a value that only lives in a setup closure would
have its icon disappear a few GC cycles after launch. Creation is factored
into `buildStatusItem`, called once at startup and again from an
`NSWorkspaceDidWakeNotification` observer, since macOS can silently drop a
status item's on-screen presence across sleep/wake even though the retained
object stays alive.

`internal/kobogateway/watcher.go` polls `FindKobos` and diffs snapshots to
emit connect/disconnect `KoboEvent`s, consumed on the main dispatch queue to
update the tooltip/menu line and fire a best-effort `UNUserNotificationCenter`
toast via raw `objc.Call` (darwinkit has no generated binding for
`UserNotifications.framework`). Both notification paths are gated by
`runningInAppBundle` (`UNUserNotificationCenter` throws with no bundle
proxy, e.g. a raw dev binary run outside `KoboGateway.app`).

## Login Item (`internal/kobogateway/loginitem.go`)

Manages a plain `~/Library/LaunchAgents` plist (not `SMAppService` — that
needs macOS 13, this app's minimum is 12.0). The plist sets
`KeepAlive.SuccessfulExit: false`, so launchd relaunches the gateway on any
abnormal exit — a panic, a listener error, or darwinkit's AppKit bridge
`SIGABRT` — but not on a clean Quit (exit 0). `EnsureInitialLoginItem`
auto-registers it once on first-ever launch (marker file); after that only
the menu-bar toggle changes it. `SyncLoginItem` runs on every launch to
rewrite an already-enabled plist to the current template (picking up changes
like `KeepAlive`) without touching `launchctl` — a bootout/bootstrap cycle
would kill the very process calling it; the refreshed policy takes effect on
the next login/reboot.

## Crash Recovery (`cmd/kobo-gateway/recover.go`)

Defense-in-depth on top of `KeepAlive`: `guard`/`recoverGo` recover and
report a panic in a single event/block (a dispatched menu update, a
notification call) to Sentry and swallow it, so one bad event doesn't take
the whole app down. `reportAndRepanic` (deferred once in `main`) instead
reports then **re-panics** — swallowing a main-thread panic would leave the
app running in a broken, un-relaunched state, whereas re-panicking exits
non-zero and lets `KeepAlive` relaunch a fresh process. `reportFatal` covers
a plain error `run()` returns (not a panic, e.g. a bind failure) — that
otherwise only goes to stderr, invisible on a console-less menu-bar app.
None of these catch darwinkit's `SIGABRT`, a native abort that bypasses Go's
panic machinery entirely — that stays covered by `KeepAlive` alone.

Crash reporting goes to Sentry (`initSentry` in `main.go`), gated on a
build-time `-ldflags -X main.SentryDSN=...` var — empty in dev
builds/`go test`, so nothing is ever sent off a dev machine.
`codecov.yml` excludes `menubar_darwin.go` from coverage since it's pure
AppKit/cgo with no window-server session under `go test`.

## Self-Update

`POST /update` and the `update` CLI subcommand both call `Updater.SelfUpdate`:
downloads `kobo-gateway-darwin-arm64` from the requesting/configured origin
(size-capped, validated as a Mach-O binary), atomically replaces the running
executable, then re-signs the `.app` bundle ad-hoc (overwriting the binary
invalidates the seal `package.sh` applied at build time) and signals a
restart. `menubar_darwin.go`'s restart path relaunches via `open -n <bundle>`
inside a real `.app` — not a bare `syscall.Exec`, which would keep the same
PID and skip LaunchServices, leaving `NSStatusItem`/`UNUserNotificationCenter`
unregistered after the swap — falling back to `syscall.Exec` only for a
bundle-less dev binary.

The web UI (`gatewayNeedsUpdate` in `web/lib/books/gatewayClient.ts`)
decides *when* to trigger this by comparing two independent things: the
gateway's `GatewayVersion` against a required floor (bump only on a real
protocol break), and its `release` build stamp against the web app's own —
both stamped with the same `github.sha` by CI, so a mismatch means a newer
binary is available even when the protocol hasn't changed. `'dev'` on either
side skips the check.

## Building

**macOS only** (cgo + Xcode command line tools — will not cross-compile from Linux):

```bash
make build   # ./bin/kobo-gateway-darwin-arm64 (arm64 native)
make dist    # packages into dist/kobo-gateway/: KoboGateway.app → .dmg, plus the raw binary
make test    # go test ./... (internal/kobogateway is pure fs/httptest, no DB)
make lint    # go vet + gofmt -l
make lint/fix
```

The Makefile pins `GOTOOLCHAIN=go1.24.13` even though `go.mod` only states
that as a minimum: darwinkit's AppKit bridge `SIGABRT`s on launch under Go
1.25+ ([progrium/darwinkit#286](https://github.com/progrium/darwinkit/issues/286),
open/unfixed). A bare `go` directive can't downgrade a newer ambient Go on
`PATH`, so every `make` target forces the exact toolchain regardless of
what's installed locally or in CI. Bump both together once the upstream
issue is fixed — never bump past 1.24.x alone, or the crash silently
reappears for anyone with a newer Go installed.

`make dist` needs `sips`/`iconutil`/`hdiutil` (standard macOS tools) to build
`AppIcon.icns` and pack the `.dmg`.

## Distribution

`kobo-gateway.dmg` and `kobo-gateway-darwin-arm64` (the self-update target)
both ship inside the single merged **app** Docker image (issue #558). Since
this module can't build on the Linux runner that builds the app image,
`.github/workflows/build-kobo-gateway.yml` builds and packages it on a
`macos-14` runner (`make dist RELEASE=${{ github.sha }} SENTRY_DSN=...`) and
uploads the result as an artifact; `docker.yml`'s app-image job downloads it
into `web/public/downloads/` before `docker build` runs — the root
`Dockerfile` just `COPY`s the two files in directly, it never builds
kobo-gateway itself. There is a separate, unrelated `gateway/` module at the
repo root (routing + process supervision for the merged api/web container,
see its own `CLAUDE.md`) — don't confuse the two. See root `CLAUDE.md`'s CI
section for the full wiring.
