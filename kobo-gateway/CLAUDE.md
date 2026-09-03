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

## Runtime

Loopback-only **HTTPS** on `127.0.0.1:41132` (`DefaultPort`): `GET /status`,
`POST /configure`, `POST /revert`, `POST /update`, all through `Server.secure()`
(host allowlist, origin allowlist, CORS + Chrome Private Network Access). HTTPS
rather than HTTP because Safari blocks a secure page from fetching a plain-HTTP
loopback URL; the cert is self-signed and trusted once via `EnsureTrusted`.

The menu bar (`menubar_darwin.go`) runs as an accessory app on the main OS
thread; `internal/kobogateway/watcher.go` polls `FindKobos` and emits
connect/disconnect events. Login-item management is a plain
`~/Library/LaunchAgents` plist whose `KeepAlive` relaunches the app on any
abnormal exit. `Updater.SelfUpdate` replaces the running executable, re-signs the
bundle, and restarts via `open -n <bundle>`.

→ [`docs/spec-kobo-gateway-runtime.md`](../docs/spec-kobo-gateway-runtime.md),
[`docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md`](../docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md)

Two things that break silently if changed:

- **The status item, its button, and its live line must stay package-level vars**, not locals — `objc.Retain` installs a Go finalizer, so a value living only in a setup closure loses its icon a few GC cycles after launch.
- **Never echo the raw `Origin` header back** — echo the matched allowlist entry, to avoid request-data forgery on the update download URL.

## Building

**macOS only** (cgo + Xcode command line tools — will not cross-compile from Linux):

```bash
make build   # ./bin/kobo-gateway-darwin-arm64 (arm64 native)
make dist    # packages into dist/kobo-gateway/: KoboGateway.app → .dmg, plus the raw binary
make test    # go test ./... (internal/kobogateway is pure fs/httptest, no DB)
make lint    # golangci-lint, shared root .golangci.yml config
make lint/fix
```

The Makefile pins `GOTOOLCHAIN=go1.24.13` on every target, even though `go.mod`
only states it as a minimum — darwinkit's AppKit bridge `SIGABRT`s on launch
under Go 1.25+. **Never bump past 1.24.x alone, or the crash silently reappears**
for anyone with a newer Go installed; bump the pin and the `go.mod` minimum
together once upstream is fixed → [`docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md`](../docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md).

`make dist` needs `sips`/`iconutil`/`hdiutil` (standard macOS tools) to build
`AppIcon.icns` and pack the `.dmg`.

## Distribution

`kobo-gateway.dmg` and `kobo-gateway-darwin-arm64` (the self-update target) are
built on a `macos-14` runner by `.github/workflows/build-kobo-gateway.yml` and
staged into the **`web`** image's `public/downloads/` by `build-web.yml`; nothing
builds kobo-gateway inside a container. Its release is compile-stamped and its
build is cacheable, so the bundled artifact's release can legitimately be older
than the rest of the deploy — which is what `KOBO_GATEWAY_RELEASE` exists to
track → [`docs/adr-0004-runtime-release-env-vs-compile-stamp.md`](../docs/adr-0004-runtime-release-env-vs-compile-stamp.md),
[`docs/adr-0002-kobo-gateway-ci-cache-split.md`](../docs/adr-0002-kobo-gateway-ci-cache-split.md).
