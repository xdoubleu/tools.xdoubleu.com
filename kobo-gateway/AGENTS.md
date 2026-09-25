# kobo-gateway/ — macOS menu-bar helper

Configures a USB-mounted Kobo for the books page. **Separate Go module** because its menu bar needs cgo + AppKit ([`darwinkit`](https://github.com/progrium/darwinkit)), which Linux builds of `api` shouldn't carry.

## Structure

```text
cmd/kobo-gateway/
  main.go            # flags, Sentry, TLS bootstrap, self-update restart loop
  bundle.go          # runningInAppBundle(): .app vs raw dev binary
  menubar_darwin.go  # real AppKit status item
  menubar_other.go   # !darwin no-op stub (keeps Linux/CI green)
  notify.go, recover.go
  assets/            # Info.plist, icons, package.sh (.app/.dmg)
internal/kobogateway/  # loopback HTTPS server; no AppKit
```

`conf.go` is the only parser/serializer of `Kobo eReader.conf` — the browser never reads it (`web/lib/books/koboConf.ts` does only string checks).

## Runtime → [`adr-0016`](../docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md)

- HTTPS on `127.0.0.1:41132` (`DefaultPort`): `GET /status`, `POST /configure|/revert|/update`, all via `Server.secure()` (host/origin allowlists, CORS, Private Network Access). HTTPS because Safari blocks secure pages fetching plain-HTTP loopback; the self-signed cert is trusted once via `EnsureTrusted`.
- `watcher.go` polls `FindKobos` for connect/disconnect. Login item is a `~/Library/LaunchAgents` plist with `KeepAlive`. `Updater.SelfUpdate` replaces the binary, re-signs, restarts via `open -n <bundle>`.
- **Status item, its button, and live line must stay package-level vars** — `objc.Retain`'s finalizer drops a closure-local one after a few GCs, losing the icon.
- **Never echo the raw `Origin` header** — echo the matched allowlist entry.

## Building (macOS only)

`make build | dist | test | lint | lint/fix`. `make dist` needs `sips`/`iconutil`/`hdiutil`. `make test` is pure fs/httptest, no DB.

**The Makefile pins `GOTOOLCHAIN=go1.24.13`** — darwinkit `SIGABRT`s on launch under Go 1.25+. Never bump past 1.24.x alone; bump the pin and the `go.mod` minimum together once upstream is fixed → [`adr-0015`](../docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md).

## Distribution

`build-kobo-gateway.yml` builds `kobo-gateway.dmg` and `kobo-gateway-darwin-arm64` (self-update target) on `macos-14`; `build-web.yml` stages them into the `web` image's `public/downloads/`. The build is cached, so its release can be older than the deploy — `KOBO_GATEWAY_RELEASE` tracks it → [`adr-0004`](../docs/adr-0004-runtime-release-env-vs-compile-stamp.md), [`adr-0002`](../docs/adr-0002-kobo-gateway-ci-cache-split.md).
