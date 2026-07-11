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

`internal/kobogateway` is a plain-Go, loopback-only HTTP server the books
page drives to read/write a USB-mounted Kobo's `Kobo eReader.conf`. It has no
AppKit dependency. Setup is gateway-only now — the browser never reads the
conf file itself, so `conf.go` is the only place that parses/serializes it
(`web/lib/books/koboConf.ts` only keeps `KOBO_DEFAULT_ENDPOINT` and
`isManagedEndpoint`, both simple string checks with no parsing to stay
compatible with). Security = strict Origin allowlist + Host check +
CORS/PNA. `POST /update` self-replaces the running binary from the
requesting origin (see "Self-update" below).

The menu bar (`menubar_darwin.go`) is purely cosmetic — a status item with a
release-version title and a Quit item — so the running app is visible and
quittable. It runs on the main OS thread (`runtime.LockOSThread` in `main`);
`serve()` in `main.go` runs the HTTP server in a goroutine and blocks the
main thread in `runUI` until Quit, a server error, or a self-update restart
signal closes the shared `stop` channel.

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

`make dist` needs `sips`/`iconutil`/`hdiutil` (all standard macOS tools) to
build `AppIcon.icns` from `assets/appicon.png` and pack the `.dmg`. Both
`assets/appicon.png` (app icon) and `assets/menubar-template.png` (status-bar
glyph) are placeholders — swap them for a designed asset; `package.sh`
regenerates the `.icns` from whatever PNG is there.

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
restart — `main.go` re-execs via `syscall.Exec`. `GatewayVersion`
(`internal/kobogateway/server.go`) must track `REQUIRED_GATEWAY_VERSION` in
`web/lib/books/gatewayClient.ts`; bump the Go side whenever the HTTP API or
file handling changes so the web UI knows to trigger the self-update.
