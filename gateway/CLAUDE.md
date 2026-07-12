# gateway/ — kobo-gateway macOS app

A menu-bar macOS helper the books page (`web/`) drives to configure a
USB-mounted Kobo e-reader. It's a **separate Go module** from `api/`
(`module tools.xdoubleu.com/gateway`) because its menu bar needs cgo and the
real AppKit/Xcode SDK ([`github.com/progrium/darwinkit`](https://github.com/progrium/darwinkit)) —
pulling that into `api/go.mod` would force every Linux build/lint/test of
the main server to carry an Objective-C dependency it never uses.

## Structure

```text
gateway/
├── cmd/kobo-gateway/
│   ├── main.go            # CLI entry, HTTP server lifecycle, self-update/restart
│   ├── menubar_darwin.go  # //go:build darwin — the real AppKit status-bar item
│   ├── menubar_other.go   # //go:build !darwin — no-op stub (keeps Linux/CI green)
│   └── assets/            # Info.plist template, icon source PNGs, package.sh
└── internal/kobogateway/  # The loopback HTTP server (routes, security, conf file I/O)
```

`internal/kobogateway` is a plain-Go, loopback-only HTTPS server the books
page drives to read/write a USB-mounted Kobo's `Kobo eReader.conf`. It has no
AppKit dependency. Setup is gateway-only now — the browser never reads the
conf file itself, so `conf.go` is the only place that parses/serializes it
(`web/lib/books/koboConf.ts` only keeps `KOBO_DEFAULT_ENDPOINT` and
`isManagedEndpoint`, both simple string checks with no parsing to stay
compatible with). Security = strict Origin allowlist + Host check +
CORS/PNA. `POST /update` self-replaces the running binary from the
requesting origin (see "Self-update" below).

HTTPS (not HTTP) is required because Safari blocks a secure page (the books
page is always `https://`) from fetching a plain-HTTP loopback URL — Chrome
exempts loopback from that check, but Safari doesn't. `tls.go` generates a
self-signed cert (`EnsureCert`) on first launch, persisted alongside a trust
marker in `~/Library/Application Support/kobo-gateway`, and `EnsureTrusted`
prompts the user once via `security add-trusted-cert` to add it to the login
keychain (skipped under `testing.Testing()` so `go test`/CI never shells
out). If Safari still rejects the cert after that prompt, the fallback is
trusting it to the System keychain instead (needs sudo).

The menu bar (`menubar_darwin.go`) shows a status item (icon from
`assets/menubar-template.png`, rendered at 18pt — set explicitly via
`Image.SetSize`, since an unsized image renders at the PNG's native 36px and
effectively disappears in the menu bar), a header naming the app and
`tools.xdoubleu.com`, an "Open tools.xdoubleu.com" link, a live Kobo
connected/disconnected status line, a "Start at Login" toggle, and Quit. It
runs on the main OS thread (`runtime.LockOSThread` in `main`); `serve()` in
`main.go` runs the HTTP server in a goroutine and blocks the main thread in
`runUI` until Quit, a server error, or a self-update restart signal closes
the shared `stop` channel.

The `NSStatusItem` is kept in a package-level `statusItem` var, not a local —
`objc.Retain` installs a Go finalizer that *releases* the object once its Go
wrapper is garbage-collected, so a value that only lives in `runUI`'s setup
closure gets finalized (and the icon disappears) a few GC cycles after
launch. A package-level reference keeps it reachable indefinitely.

`internal/kobogateway/watcher.go`'s `Watch` polls `FindKobos` every
`koboPollInterval` (main.go) and diffs snapshots (`DiffKobos`) to emit
connect/disconnect `KoboEvent`s; `menubar_darwin.go` consumes them off the
main dispatch queue to update the tooltip/menu line and fire a best-effort
`NSUserNotification` toast (darwinkit has no generated binding for it — it's
deprecated — or for `UNUserNotificationCenter`, which needs a properly
signed bundle/entitlement this ad-hoc-signed `.app` doesn't have; see the
`postNotification` comment for the `objc.Call` fallback, mirroring
darwinkit's own notification example). `KoboTooltip` prefixes the tooltip
with the running release so the version is visible on hover without opening
the menu.

`internal/kobogateway/loginitem.go` manages a `~/Library/LaunchAgents`
plist (a plain LaunchAgent, not `SMAppService` — the latter needs macOS 13,
this app's `LSMinimumSystemVersion` is 12.0) so the gateway starts at login.
`EnsureInitialLoginItem` auto-registers it once on first-ever launch
(tracked by a marker file next to the TLS cert in
`~/Library/Application Support/kobo-gateway`); after that, only the
menu-bar toggle changes it. `main.go`'s `serve()` only calls it outside
`testing.Testing()`, alongside the `runUI` gate — otherwise `go test` would
write a real LaunchAgent into the test-runner's actual home directory.

## Building

**macOS only** (cgo + Xcode command line tools required — this will not
cross-compile from Linux):

```bash
make build   # ./bin/kobo-gateway-darwin-arm64 (arm64 native, no cross-compile)
make dist    # packages into dist/gateway/: KoboGateway.app→.dmg + the raw binary
make test    # go test ./... (internal/kobogateway is pure fs/httptest, no DB)
make lint    # go vet + gofmt -l
make lint/fix
```

The Makefile pins `GOTOOLCHAIN=go1.24.13` — darwinkit's AppKit bridge
`SIGABRT`s on launch under Go 1.25+
([progrium/darwinkit#286](https://github.com/progrium/darwinkit/issues/286),
open/unfixed), so `go.mod`'s `go 1.24.13` line alone isn't enough (a bare
`go` directive only sets a minimum and won't downgrade a newer ambient `go`
on `PATH`). Every `make` target in this module therefore builds/tests with
exactly that toolchain regardless of what's installed locally or in CI. Bump
both once the upstream issue is fixed — do not bump go.mod's `go` line past
1.24.x on its own, it'll silently reintroduce the crash for anyone with a
newer Go installed.

`make dist` needs `sips`/`iconutil`/`hdiutil` (all standard macOS tools) to
build `AppIcon.icns` from `assets/appicon.png` and pack the `.dmg`.
`assets/appicon.png` (app icon) and `assets/menubar-template.png`
(status-bar glyph) share the same e-reader glyph so the two stay visually
consistent; `package.sh` regenerates the `.icns` from whatever PNG is there,
so swapping in a redesigned asset needs no other change.

## Distribution

`kobo-gateway.dmg` (download button) and `kobo-gateway-darwin-arm64` (raw
binary, self-update target) both ship inside the **web** Docker image,
served at `/downloads/kobo-gateway.dmg` and
`/downloads/kobo-gateway-darwin-arm64`. Since this module can't build on the
Linux runner that builds the web image, `.github/workflows/build-gateway.yml`
builds and packages it on a `macos-14` runner and hands both files to
`docker.yml` as an `actions/upload-artifact` artifact; `web/Dockerfile`
`COPY`s them in directly (there is no gateway build stage in that Dockerfile).
See the root `CLAUDE.md` CI section for the full wiring.

## Self-update

`POST /update` and the `update` CLI subcommand both call
`Updater.SelfUpdate`, which downloads `kobo-gateway-darwin-arm64` from the
requesting/configured origin and atomically replaces the running executable
(inside an app bundle, that's `Contents/MacOS/kobo-gateway`), then signals a
restart — `main.go` re-execs via `syscall.Exec`.

The web UI (`KoboGatewaySetup.tsx`) decides *when* to trigger this via
`gatewayNeedsUpdate` (`web/lib/books/gatewayClient.ts`), which compares two
independent things:

- **`GatewayVersion`** (`internal/kobogateway/server.go`) vs.
  `REQUIRED_GATEWAY_VERSION` — a floor for genuine HTTP API/file-handling
  breaks; bump both together only when the protocol itself changes ("routine
  releases don't bump it").
- **`release`** (the `/status` field, `-ldflags -X main.Release`) vs. the web
  app's own `getRelease()` — both are stamped with the same `github.sha` by
  CI (`build-gateway.yml`'s `make dist RELEASE=${{ github.sha }}`, chained to
  the same web build), so any mismatch means a newer gateway binary is
  available. This is what actually delivers *routine* releases (bug fixes
  that don't touch the protocol) to installed gateways — without it, a
  gateway only updates on the rare protocol bump. `'dev'` on either side
  skips the check (no deployed binary to fetch).
