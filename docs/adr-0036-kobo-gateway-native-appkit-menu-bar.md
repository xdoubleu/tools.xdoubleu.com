# ADR-0036: kobo-gateway's menu bar is hand-written Objective-C behind cgo

- Status: Accepted
- Issues: #2613, [progrium/darwinkit#286](https://github.com/progrium/darwinkit/issues/286)
- Affects: `kobo-gateway/cmd/kobo-gateway/menubar_darwin.{go,m,h}`, `kobo-gateway/go.mod`, `kobo-gateway/Makefile`, `Info.plist`
- Supersedes: ADR-0015's toolchain pin

## Context

darwinkit `SIGABRT`s under Go 1.25+ and has been dormant since 2024-07, so
ADR-0015 pinned the gateway to Go 1.24. That Go is end-of-life, and
govulncheck reported 26 reachable vulnerabilities in a binary that serves
HTTPS to browser pages.

## Decision

- **A small Objective-C file (`menubar_darwin.m`, ARC) owns all AppKit and
  UserNotifications calls**, behind a C header. Go calls it through cgo, and
  it calls back into `//export` functions. Login items, conf handling and
  the server stay in Go.
- **Main thread only**: `init` locks the main goroutine to the main OS
  thread, `[NSApp run]` runs there, and Go hands work to it through
  `dispatch_async` on the main queue (`runOnMain`, a `cgo.Handle`).
- **No toolchain pin**: `go.mod` tracks the repo's Go via Renovate. Go 1.27
  requires macOS 13, so `LSMinimumSystemVersion` is 13.0.
- The stub builds for `!darwin || !cgo`. `make build` forces `CGO_ENABLED=1`,
  so a release can't ship that stub.

## Alternatives considered

- **Fork darwinkit and fix #286**: keeps a large generated binding layer for
  six AppKit classes.
- **Another Go binding (e.g. a systray library)**: a new dependency for the
  same small surface, with no login-item or notification parity.

## Consequences

- The Objective-C compiles only on macOS: CI (`macos-26`) is its build check,
  and behaviour is verified by a manual smoke test.
- An Objective-C exception escaping into Go aborts the process; the bridge
  catches the ones UserNotifications throws outside a `.app`.
- ADR-0016's "revisit when minimum macOS rises to 13" now applies:
  `SMAppService` is available, but the LaunchAgents plist is kept.

## Revisit when

The menu bar needs more than a status item, an alert and notifications.
