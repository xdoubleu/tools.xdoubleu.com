# ADR-0016: Loopback HTTPS for Safari, and a LaunchAgents plist over `SMAppService`

- Status: Accepted
- Issues: —
- Affects: `kobo-gateway/internal/kobogateway/server.go`, `tls.go`, `loginitem.go`

## Context

Two macOS constraints: Safari blocks an HTTPS page from fetching a plain-HTTP
loopback URL (Chrome exempts loopback), and `SMAppService` needs macOS 13 while
the app supports 12.0.

## Decision

- **Listen on `https://127.0.0.1:41132`.** `EnsureCert` generates a self-signed
  ECDSA P-256 cert (10 years) and `EnsureTrusted` adds it to the login keychain
  via `security add-trusted-cert`, prompting once. Files live in
  `~/Library/Application Support/kobo-gateway`; delete `cert.pem`/`key.pem` to
  regenerate.
- **Plain `~/Library/LaunchAgents` plist** with
  `KeepAlive.SuccessfulExit: false`: launchd relaunches on any abnormal exit,
  including darwinkit's `SIGABRT`, but not on a clean Quit.
  `EnsureInitialLoginItem` registers it once on first launch; afterwards only
  the menu toggle changes it. `SyncLoginItem` rewrites an enabled plist each
  launch **without `launchctl`**, since bootout would kill the caller.

## Alternatives considered

- **Plain HTTP** — Safari refuses it.
- **`SMAppService`** — needs macOS 13.
- **`launchctl bootout`/`bootstrap` to apply changes** — kills the syncing process.

## Consequences

- One-time keychain prompt; if Safari still rejects the cert, the fallback is the
  System keychain (needs sudo).
- Plist policy changes apply at next login.

## Revisit when

Minimum macOS rises to 13.
